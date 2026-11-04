package usecase

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"gorm.io/gorm"
)

// Inventory is a database operation, not a top-k semantic search. Restrict this
// route to explicit catalog requests; uploaded data and revisions retain their
// original workflow rather than being silently discarded.
func assistantCatalogRequest(req domain.AssistantSubmit, checkpoint domain.JSONB) bool {
	if req.BaseArtifactID != "" || len(req.AttachmentIDs) > 0 || req.WebSearchEnabled {
		return false
	}
	text := strings.ToLower(req.Message + "\n" + assistantString(checkpoint["feedback"]))
	if req.Format != "" && req.Format != "xlsx" {
		return false
	}
	return (req.Format == "xlsx" || strings.Contains(text, "excel") || strings.Contains(text, "xlsx")) &&
		strings.Contains(text, "知识库") &&
		(strings.Contains(text, "目录册") || strings.Contains(text, "文档清单") || strings.Contains(text, "文件清单") || strings.Contains(text, "基本信息") || strings.Contains(text, "文件名称"))
}

type assistantCatalogAccess struct {
	nodes    map[string]*domain.Node
	navs     map[string]domain.Nav
	readable map[string]bool
}

func (u *AssistantUsecase) catalogAccess(ctx context.Context, kb, user string) (*assistantCatalogAccess, error) {
	var nodes []*domain.Node
	// Never pull every document body into the worker just to export metadata.
	if err := u.db.WithContext(ctx).Select("id,kb_id,nav_id,parent_id,type,name,permissions,rag_info,created_at,updated_at").Where("kb_id=?", kb).Find(&nodes).Error; err != nil {
		return nil, err
	}
	var navs []domain.Nav
	if err := u.db.WithContext(ctx).Where("kb_id=?", kb).Find(&navs).Error; err != nil {
		return nil, err
	}
	access, err := u.chat.enterpriseUsecase.BuildShareAccessContext(ctx, user)
	if err != nil {
		return nil, err
	}
	a := &assistantCatalogAccess{nodes: map[string]*domain.Node{}, navs: map[string]domain.Nav{}, readable: map[string]bool{}}
	for _, n := range nodes {
		a.nodes[n.ID] = n
	}
	for _, n := range navs {
		a.navs[n.ID] = n
	}
	resolve := u.chat.enterpriseUsecase.newTreePermissionResolver(ctx, a.nodes)
	for _, n := range nodes {
		// Broken/cross-KB ancestry fails closed; do not fetch another KB's path.
		seen := map[string]bool{n.ID: true}
		valid := true
		for parent := n.ParentID; parent != ""; {
			if a.isNavRoot(parent) {
				// Historical uploads can use nav:<id> as their parent. It is
				// a valid hierarchy boundary, not a missing document folder.
				a.nodes[parent] = nil
				break
			}
			p := a.nodes[parent]
			if p == nil || seen[parent] {
				valid = false
				break
			}
			seen[parent] = true
			parent = p.ParentID
		}
		if !valid {
			continue
		}
		perm, err := resolve(n.ID, n.ParentID, n.Permissions)
		if err != nil {
			return nil, err
		}
		if n.NavId != "" {
			nav, ok := a.navs[n.NavId]
			if !ok {
				continue
			}
			perm = u.chat.enterpriseUsecase.MergeNodeAndNavPermissions(perm, nav.Permissions)
		}
		// A catalog needs reading permission, not a successful vector index or
		// permission to use the document in question answering.
		a.readable[n.ID] = u.chat.enterpriseUsecase.CanVisitable(perm, access)
	}
	return a, nil
}

func (a *assistantCatalogAccess) isNavRoot(id string) bool {
	_, ok := a.navs[strings.TrimPrefix(id, "nav:")]
	return ok
}

func (u *AssistantUsecase) checkCatalogSources(ctx context.Context, kb, user string, sources domain.JSONB) error {
	var a *assistantCatalogAccess
	for _, s := range sourceList(sources) {
		if s.Kind != "catalog_node" {
			continue
		}
		if a == nil {
			var err error
			a, err = u.catalogAccess(ctx, kb, user)
			if err != nil {
				return err
			}
		}
		if !a.readable[s.ID] {
			return ErrAssistantAccess
		}
	}
	return nil
}

type assistantCatalogEntry struct {
	NodeID, NavID string
	domain.NodeRelease
}

func (u *AssistantUsecase) catalogEntries(ctx context.Context, r *domain.AssistantRun, req domain.AssistantSubmit) ([]assistantCatalogEntry, *assistantCatalogAccess, error) {
	a, err := u.catalogAccess(ctx, r.KBID, r.UserID)
	if err != nil {
		return nil, nil, err
	}
	var entries []assistantCatalogEntry
	query := u.db.WithContext(ctx).Table("node_releases nr").Select("nr.id,nr.node_id,nr.kb_id,nr.name,nr.type,nr.meta,nr.parent_id,nr.created_at,nr.updated_at,n.position,n.nav_id").Joins("JOIN nodes n ON n.id=nr.node_id AND n.kb_id=nr.kb_id").Where("nr.kb_id=?", r.KBID)
	if req.SourceScope.Type == "documents" {
		ids := []string{}
		for _, id := range req.SourceScope.NodeIDs {
			if !a.readable[id] || req.SourceScope.ReleaseIDs[id] == "" {
				return nil, nil, ErrAssistantAccess
			}
			ids = append(ids, req.SourceScope.ReleaseIDs[id])
		}
		if len(ids) == 0 {
			return nil, a, nil
		}
		query = query.Where("nr.id IN ? AND nr.node_id IN ?", ids, req.SourceScope.NodeIDs)
	} else {
		var release domain.KBRelease
		if err = u.db.WithContext(ctx).Where("kb_id=?", r.KBID).Order("created_at DESC").First(&release).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, a, nil
			}
			return nil, nil, err
		}
		// Same published snapshot as the user-facing tree, not unpublished drafts
		// or every historical release of the same document.
		query = query.Joins("JOIN kb_release_node_releases kr ON kr.node_release_id=nr.id AND kr.node_id=nr.node_id AND kr.kb_id=nr.kb_id").Where("kr.release_id=?", release.ID)
	}
	if err = query.Order("n.position,n.id").Scan(&entries).Error; err != nil {
		return nil, nil, err
	}
	result := make([]assistantCatalogEntry, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		if !a.readable[entry.NodeID] || seen[entry.NodeID] {
			continue
		}
		seen[entry.NodeID] = true
		result = append(result, entry)
	}
	return result, a, nil
}

func catalogPath(entry assistantCatalogEntry, published map[string]assistantCatalogEntry, a *assistantCatalogAccess) string {
	parts := []string{}
	seen := map[string]bool{entry.NodeID: true}
	for id := entry.ParentID; id != ""; {
		if a.isNavRoot(id) {
			break
		}
		if seen[id] {
			break
		}
		seen[id] = true
		p, ok := published[id]
		if !ok || !a.readable[id] {
			parts = append(parts, "受限或未发布目录")
			break
		}
		parts = append(parts, p.Name)
		id = p.ParentID
	}
	if nav, ok := a.navs[entry.NavID]; ok {
		parts = append(parts, nav.Name)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, " / ")
}

// HEAD only, bounded parallelism and a single overall deadline. Missing legacy
// objects never prevent export or turn an unknown size into zero bytes.
func (u *AssistantUsecase) catalogSizes(ctx context.Context, entries []assistantCatalogEntry) map[string]int64 {
	result := map[string]int64{}
	if u.storage == nil {
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	keys := map[string]bool{}
	for _, e := range entries {
		if e.Type == domain.NodeTypeDocument && e.Meta.FileKey != "" {
			keys[e.Meta.FileKey] = true
		}
	}
	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range jobs {
				probe, done := context.WithTimeout(ctx, 3*time.Second)
				stat, err := u.storage.StatObject(probe, key)
				done()
				if err == nil && stat.Size >= 0 {
					mu.Lock()
					result[key] = stat.Size
					mu.Unlock()
				}
			}
		}()
	}
loop:
	for key := range keys {
		select {
		case jobs <- key:
		case <-ctx.Done():
			break loop
		}
	}
	close(jobs)
	wg.Wait()
	return result
}

func (u *AssistantUsecase) buildAssistantCatalog(ctx context.Context, r *domain.AssistantRun, req domain.AssistantSubmit) error {
	if err := u.stage(ctx, r, "读取知识库完整目录", nil); err != nil {
		return err
	}
	entries, access, err := u.catalogEntries(ctx, r, req)
	if err != nil {
		return err
	}
	r.Checkpoint["catalog_entries"] = len(entries)
	if err = u.stage(ctx, r, "核对文件大小", map[string]any{"checkpoint": r.Checkpoint}); err != nil {
		return err
	}
	sizes := u.catalogSizes(ctx, entries)
	if err = ctx.Err(); err != nil {
		return err
	}
	spec, sources := assistantCatalogSpec(entries, access, sizes, time.Now())
	r.Sources = sourcesJSON(sources)
	// This route has not used the old model's partial retrieval or unverified
	// outline. Preserve diagnostics, but retire that plan from the delivered card.
	delete(r.Checkpoint, "planning")
	r.Checkpoint["catalog_export"] = true
	r.Checkpoint["spec"] = spec
	if err = u.stage(ctx, r, "生成完整目录册", map[string]any{"checkpoint": r.Checkpoint, "sources": r.Sources}); err != nil {
		return err
	}
	u.logger.Info("assistant catalog snapshot ready", log.String("run_id", r.ID), log.Int("documents", len(sources)))
	return u.renderAssistantArtifact(ctx, r, req, spec, nil, nil)
}

func assistantCatalogSpec(entries []assistantCatalogEntry, access *assistantCatalogAccess, sizes map[string]int64, now time.Time) (domain.JSONB, []domain.AssistantSource) {
	published := map[string]assistantCatalogEntry{}
	for _, e := range entries {
		published[e.NodeID] = e
	}
	rows := [][]any{}
	sources := []domain.AssistantSource{}
	missing := 0
	for _, e := range entries {
		if e.Type != domain.NodeTypeDocument {
			continue
		}
		fileName := e.Meta.FileName
		if fileName == "" {
			fileName = e.Name
		}
		kind := e.Meta.FileType
		if kind == "" {
			kind = strings.TrimPrefix(path.Ext(fileName), ".")
		}
		if kind == "" {
			kind = "在线文档"
		}
		var size any = nil
		sizeState := "在线文档，无原文件"
		if e.Meta.FileKey != "" {
			if v, ok := sizes[e.Meta.FileKey]; ok {
				size = v
				sizeState = "已核对原文件"
			} else {
				missing++
				sizeState = "未能获取原文件大小"
			}
		}
		n := access.nodes[e.NodeID]
		learning := map[string]string{"PENDING": "等待学习", "RUNNING": "学习中", "FAILED": "学习失败", "SUCCEEDED": "学习成功", "REINDEX": "重新学习中"}[string(n.RagInfo.Status)]
		if learning == "" {
			learning = "未记录"
		}
		rows = append(rows, []any{e.Name, fileName, strings.ToLower(kind), catalogPath(e, published, access), size, sizeState, "已发布", learning, e.Meta.Summary, e.CreatedAt.Format(time.RFC3339), e.UpdatedAt.Format(time.RFC3339), e.NodeID, e.ID, "/node/" + e.NodeID})
		sources = append(sources, domain.AssistantSource{Kind: "catalog_node", ID: e.NodeID, Version: e.ID, Name: e.Name})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := fmt.Sprint(rows[i][3]), fmt.Sprint(rows[j][3])
		if a != b {
			return a < b
		}
		return fmt.Sprint(rows[i][0], rows[i][11]) < fmt.Sprint(rows[j][0], rows[j][11])
	})
	for i, row := range rows {
		rows[i] = append([]any{i + 1}, row...)
	}
	columns := []string{"序号", "文档名称", "文件名称", "文件类型", "所属目录", "文件大小（字节）", "大小核对状态", "发布状态", "学习状态", "已记录摘要", "版本创建时间", "版本更新时间", "文档ID", "发布版本ID", "用户端相对路径"}
	sheets := []domain.JSONB{{"name": "导出说明", "columns": []string{"项目", "说明"}, "rows": [][]any{{"导出时间", now.Format(time.RFC3339)}, {"范围", "当前账号有权查阅的已发布资料；指定资料时仅导出指定范围"}, {"文档总数", len(rows)}, {"大小未获取数", missing}, {"分类依据", "实际栏目及文件夹路径，不使用模型猜测分类"}, {"文件大小", "原文件对象的字节数；空值表示没有原文件或读取不到，不表示0字节"}, {"摘要", "仅使用已记录的摘要，未记录时留空"}, {"学习状态", "当前索引状态；没有索引的资料仍会列入目录"}, {"后续权限变化", "下载和预览时重新检查本次目录所涉及文档的查阅权限"}}}}
	for start := 0; start < len(rows); start += 1000 {
		end := min(start+1000, len(rows))
		sheets = append(sheets, domain.JSONB{"name": fmt.Sprintf("文档目录%d", start/1000+1), "columns": columns, "rows": rows[start:end]})
	}
	if len(rows) == 0 {
		sheets = append(sheets, domain.JSONB{"name": "文档目录", "columns": columns, "rows": [][]any{}})
	}
	answer := fmt.Sprintf("已按实际目录整理 %d 份有权查阅的已发布文档，包含文件名称、类型、目录、大小和版本信息。", len(rows))
	if missing > 0 {
		answer += fmt.Sprintf("其中 %d 份原文件暂未取到大小，已在表中标注；没有用估算值代替。", missing)
	}
	return domain.JSONB{"format": "xlsx", "title": "知识库文档目录册", "answer": answer, "sheets": sheets, "catalog_export": true}, sources
}
