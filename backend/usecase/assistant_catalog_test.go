package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
)

func TestAssistantCatalogIntentAndPagination(t *testing.T) {
	request := domain.AssistantSubmit{Message: "给我整理一下目前的知识库，做一个图书馆目录册，做成Excel格式"}
	if !assistantCatalogRequest(request, nil) {
		t.Fatal("catalog intent missed")
	}
	for _, req := range []domain.AssistantSubmit{{Message: "根据知识库生成产品介绍Excel"}, {Message: request.Message, BaseArtifactID: "revision"}, {Message: request.Message, AttachmentIDs: []string{"uploaded"}}, {Message: request.Message, Format: "pptx"}} {
		if assistantCatalogRequest(req, nil) {
			t.Fatal("unrelated task or explicit inputs discarded")
		}
	}
	a := &assistantCatalogAccess{nodes: map[string]*domain.Node{}, navs: map[string]domain.Nav{"nav": {Name: "已发布栏目"}}, readable: map[string]bool{}}
	entries := []assistantCatalogEntry{{NodeID: "folder", NavID: "nav", NodeRelease: domain.NodeRelease{Name: "已发布目录", Type: domain.NodeTypeFolder}}}
	a.readable["folder"] = true
	for i := 0; i < 2005; i++ {
		id := fmt.Sprint(i)
		a.nodes[id] = &domain.Node{ID: id}
		a.readable[id] = true
		entries = append(entries, assistantCatalogEntry{NodeID: id, NavID: "nav", NodeRelease: domain.NodeRelease{ID: "release-" + id, Name: "同名文件.xlsx", Type: domain.NodeTypeDocument, ParentID: "folder", Meta: domain.NodeMeta{FileKey: "missing"}}})
	}
	spec, sources := assistantCatalogSpec(entries, a, nil, time.Now())
	if len(sources) != 2005 {
		t.Fatal("same-name files were collapsed")
	}
	if err := prepareArtifactSpec(spec, "xlsx", sourcesJSON(sources), nil); err != nil {
		t.Fatal(err)
	}
	sheets := assistantArray(spec["sheets"])
	if len(sheets) != 5 {
		t.Fatalf("not paginated: %d", len(sheets))
	}
	total := 0
	for _, sheet := range sheets {
		if strings.HasPrefix(assistantString(sheet["name"]), "文档目录") {
			var v struct {
				Rows [][]any `json:"rows"`
			}
			_ = assistantDecode(sheet, &v)
			if len(v.Rows) > 1000 {
				t.Fatal("renderer row limit exceeded")
			}
			total += len(v.Rows)
			for _, row := range v.Rows {
				if row[5] != nil || row[4] != "已发布栏目 / 已发布目录" {
					t.Fatalf("fabricated size or path: %+v", row)
				}
			}
		}
	}
	if total != 2005 {
		t.Fatal("silently truncated inventory")
	}
	a.readable["folder"] = false
	if strings.Contains(catalogPath(entries[1], map[string]assistantCatalogEntry{"folder": entries[0]}, a), "已发布目录") {
		t.Fatal("restricted ancestor name leaked")
	}
}

func TestAssistantCatalogPublishedScopeAndRevocationPostgres(t *testing.T) {
	u, db, r := planningFixture(t)
	ctx := context.Background()
	// A non-super administrator is subject to the same document read policy.
	if err := db.Model(&domain.User{}).Where("id=?", r.UserID).Update("role", consts.UserRoleUser).Error; err != nil {
		t.Fatal(err)
	}
	nav := domain.Nav{ID: uuid.NewString(), KbID: r.KBID, Name: "公开栏目"}
	if err := db.Create(&nav).Error; err != nil {
		t.Fatal(err)
	}
	release := domain.KBRelease{ID: uuid.NewString(), KBID: r.KBID}
	if err := db.Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	versions := map[string]string{}
	for i := 0; i < 17; i++ {
		n := domain.Node{ID: uuid.NewString(), KBID: r.KBID, NavId: nav.ID, Type: domain.NodeTypeDocument, Name: "不应导出的草稿名称"}
		if i == 15 {
			n.Permissions = domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{uuid.NewString()}}
		}
		if i == 0 {
			n.ParentID = "nav:" + nav.ID
		}
		if err := db.Create(&n).Error; err != nil {
			t.Fatal(err)
		}
		nr := domain.NodeRelease{ID: uuid.NewString(), KBID: r.KBID, NodeID: n.ID, ParentID: n.ParentID, Name: fmt.Sprintf("已发布文件%d.mp4", i), Type: domain.NodeTypeDocument, Meta: domain.NodeMeta{Extraction: &domain.FileExtraction{Scope: "metadata"}}}
		if err := db.Create(&nr).Error; err != nil {
			t.Fatal(err)
		}
		// Last document is an unpublished release; never appears in the portal.
		if i != 16 {
			if err := db.Create(&domain.KBReleaseNodeRelease{ID: uuid.NewString(), KBID: r.KBID, ReleaseID: release.ID, NodeID: n.ID, NodeReleaseID: nr.ID, NavID: nav.ID}).Error; err != nil {
				t.Fatal(err)
			}
		}
		ids = append(ids, n.ID)
		versions[n.ID] = nr.ID
	}
	entries, a, err := u.catalogEntries(ctx, r, domain.AssistantSubmit{SourceScope: domain.AssistantScope{Type: "all"}})
	if err != nil || len(entries) != 15 {
		t.Fatalf("full readable inventory: got=%d err=%v", len(entries), err)
	}
	spec, sources := assistantCatalogSpec(entries, a, nil, time.Now())
	if len(sources) != 15 || strings.Contains(fmt.Sprint(spec), "草稿名称") {
		t.Fatal("published metadata not respected")
	}
	if err = u.checkSources(ctx, r.KBID, r.UserID, sourcesJSON(sources)); err != nil {
		t.Fatal(err)
	}
	selected, _, err := u.catalogEntries(ctx, r, domain.AssistantSubmit{SourceScope: domain.AssistantScope{Type: "documents", NodeIDs: ids[:1], ReleaseIDs: versions}})
	if err != nil || len(selected) != 1 {
		t.Fatalf("explicit scope expanded: %d %v", len(selected), err)
	}
	if err = db.Model(&domain.Node{}).Where("id=?", ids[0]).Update("permissions", domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{uuid.NewString()}}).Error; err != nil {
		t.Fatal(err)
	}
	if u.checkSources(ctx, r.KBID, r.UserID, sourcesJSON(sources)) == nil {
		t.Fatal("permission revocation ignored")
	}
}
