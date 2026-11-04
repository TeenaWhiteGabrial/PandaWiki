package usecase

import (
	"context"
	"os"
	"testing"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
)

// Explicit local opt-in. Read-only queries inside a rolled-back transaction;
// no model calls, object writes, user changes or task submissions.
func TestAssistantCatalogLocalSnapshot(t *testing.T) {
	id := os.Getenv("PANDA_WIKI_TEST_CATALOG_RUN")
	if id == "" {
		t.Skip("set a local catalog run ID for read-only policy comparison")
	}
	db, logger := chatContextDB(t)
	var run domain.AssistantRun
	if err := db.First(&run, "id=?", id).Error; err != nil {
		t.Fatal(err)
	}
	nodeRepo := pg.NewNodeRepository(db, logger)
	u := &AssistantUsecase{db: db.DB, logger: logger, chat: &ChatUsecase{nodeRepo: nodeRepo, enterpriseUsecase: &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(db, logger), nodeRepo: nodeRepo, navRepo: pg.NewNavRepository(db, logger)}}}
	ctx := context.Background()
	entries, a, err := u.catalogEntries(ctx, &run, domain.AssistantSubmit{SourceScope: domain.AssistantScope{Type: "all"}})
	if err != nil {
		t.Fatal(err)
	}
	actual := map[string]bool{}
	for _, entry := range entries {
		if entry.Type == domain.NodeTypeDocument {
			actual[entry.NodeID] = true
		}
	}
	var published []domain.Node
	if err = db.Raw(`SELECT n.* FROM nodes n JOIN kb_release_node_releases kr ON kr.node_id=n.id AND kr.kb_id=n.kb_id JOIN node_releases nr ON nr.id=kr.node_release_id WHERE n.kb_id=? AND nr.type=2 AND kr.release_id=(SELECT id FROM kb_releases WHERE kb_id=? ORDER BY created_at DESC LIMIT 1)`, run.KBID, run.KBID).Scan(&published).Error; err != nil {
		t.Fatal(err)
	}
	access, err := u.chat.enterpriseUsecase.BuildShareAccessContext(ctx, run.UserID)
	if err != nil {
		t.Fatal(err)
	}
	expected := 0
	for _, n := range published {
		perms, err := u.chat.enterpriseUsecase.ResolveNodePermissions(ctx, &n)
		if err != nil {
			t.Fatal(err)
		}
		if nav, ok := a.navs[n.NavId]; ok {
			perms = u.chat.enterpriseUsecase.MergeNodeAndNavPermissions(perms, nav.Permissions)
		} else if n.NavId != "" {
			continue
		}
		readable := u.chat.enterpriseUsecase.CanVisitable(perms, access)
		if readable {
			expected++
		}
		if actual[n.ID] != readable {
			t.Fatalf("catalog policy differs from document-read policy for node %s", n.ID)
		}
	}
	t.Logf("published=%d canonical-readable=%d catalog=%d", len(published), expected, len(actual))
}
