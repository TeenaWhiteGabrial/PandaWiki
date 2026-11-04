package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
)

func assistantArray(v any) []domain.JSONB {
	b, _ := json.Marshal(v)
	var out []domain.JSONB
	_ = json.Unmarshal(b, &out)
	return out
}

func assistantArtifactOutline(a *domain.AssistantArtifact) []domain.AssistantArtifactPart {
	result := []domain.AssistantArtifactPart{}
	unit := map[string]string{"pptx": "页", "docx": "节", "xlsx": "表"}[a.Format]
	for _, part := range assistantArray(a.Spec[assistantArtifactPartKey(a.Format)]) {
		id := assistantString(part["id"])
		if id == "" || id == "system-sources" {
			continue
		}
		title := assistantString(part["title"])
		if a.Format == "xlsx" {
			title = assistantString(part["name"])
		}
		result = append(result, domain.AssistantArtifactPart{ID: id, Title: title, Position: len(result) + 1, Unit: unit})
	}
	return result
}
func prepareArtifactSpec(spec domain.JSONB, format string, sources domain.JSONB, base *domain.AssistantArtifact) error {
	key := "sections"
	if format == "pptx" {
		key = "slides"
	}
	if format == "xlsx" {
		key = "sheets"
	}
	original := assistantArray(spec[key])
	if len(original) == 0 {
		return fmt.Errorf("文件内容为空，请重新生成")
	}
	baseIDs := map[string]bool{}
	if base != nil {
		for _, item := range assistantArray(base.Spec[key]) {
			if id, ok := item["id"].(string); ok {
				baseIDs[id] = true
			}
		}
	}
	items := []domain.JSONB{}
	seen := map[string]bool{}
	for _, item := range original {
		if id, _ := item["id"].(string); id == "system-sources" {
			continue
		}
		id, _ := item["id"].(string)
		if id == "" || !baseIDs[id] {
			id = uuid.NewString()
		}
		if seen[id] {
			return fmt.Errorf("文件修订目标重复，请重新生成")
		}
		seen[id] = true
		item["id"] = id
		items = append(items, item)
	}
	refs := sourceList(sources)
	if len(refs) > 0 {
		lines := []string{}
		rows := [][]any{}
		for _, s := range refs {
			location := s.Version
			if s.URL != "" {
				location = s.URL
			}
			lines = append(lines, s.Name+" · "+location)
			rows = append(rows, []any{s.Kind, s.Name, location})
		}
		switch format {
		case "docx":
			items = append(items, domain.JSONB{"id": "system-sources", "title": "资料来源", "paragraphs": lines})
		case "pptx":
			if len(items) >= 20 {
				return fmt.Errorf("PPT需预留一页显示资料来源，请将正文限制为19页")
			}
			bullets := []string{}
			for _, s := range refs[:min(4, len(refs))] {
				name := []rune(s.Name)
				bullets = append(bullets, string(name[:min(60, len(name))]))
			}
			bullets = append(bullets, fmt.Sprintf("实际使用%d项来源，完整记录见会话文件卡片。", len(refs)))
			items = append(items, domain.JSONB{"id": "system-sources", "title": "资料来源", "bullets": bullets})
		case "xlsx":
			catalog, _ := spec["catalog_export"].(bool)
			if catalog {
				// IDs and pinned releases are already present in each catalog row;
				// keep the complete dependency set privately for permission checks.
				rows = [][]any{{"知识库目录", fmt.Sprintf("共%d份资料，逐份文档ID及发布版本见文档目录工作表", len(refs)), "下载和预览时重新检查权限"}}
			}
			if (!catalog && len(items) >= 5) || len(items) >= 24 {
				return fmt.Errorf("Excel需预留来源工作表，请将数据表限制为4个")
			}
			name := "资料来源"
			for _, s := range items {
				if s["name"] == name {
					name = "系统资料来源"
				}
			}
			items = append(items, domain.JSONB{"id": "system-sources", "name": name, "columns": []string{"来源类型", "名称", "地址或版本"}, "rows": rows})
		}
	}
	spec[key] = items
	return nil
}

// Image bytes enter the renderer only after owner/source checks. No model URL,
// filesystem path, data URI or executable asset is accepted.
func (u *AssistantUsecase) imageAssets(ctx context.Context, r *domain.AssistantRun, spec domain.JSONB) (map[string]string, error) {
	allowed := map[string]bool{}
	for _, s := range sourceList(r.Sources) {
		if s.Kind == "upload" {
			allowed[s.ID] = true
		}
	}
	ids := map[string]bool{}
	for _, sec := range assistantArray(spec["sections"]) {
		for _, image := range assistantArray(sec["images"]) {
			if id, ok := image["upload_id"].(string); ok {
				ids[id] = true
			}
		}
	}
	for _, slide := range assistantArray(spec["slides"]) {
		if image, ok := slide["image"].(map[string]any); ok {
			if id, ok := image["upload_id"].(string); ok {
				ids[id] = true
			}
		}
	}
	result := map[string]string{}
	var total int
	for id := range ids {
		if !allowed[id] {
			return nil, fmt.Errorf("图片不属于本轮授权素材")
		}
		v, e := u.GetUpload(ctx, r.KBID, r.UserID, id)
		if e != nil {
			return nil, e
		}
		if v.Format != "png" && v.Format != "jpg" && v.Format != "jpeg" {
			return nil, fmt.Errorf("图片素材只支持PNG/JPEG")
		}
		b, e := u.objectBytes(ctx, v.ObjectKey, assistantMaxFile)
		if e != nil {
			return nil, e
		}
		if digest(b) != v.Hash {
			return nil, fmt.Errorf("图片素材校验失败")
		}
		total += len(b)
		if total > 10<<20 {
			return nil, fmt.Errorf("图片总计超过10 MB，请压缩素材")
		}
		result[id] = base64.StdEncoding.EncodeToString(b)
	}
	return result, nil
}
