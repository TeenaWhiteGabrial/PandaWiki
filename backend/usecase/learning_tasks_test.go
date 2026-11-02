package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestLearningTaskPagingKeepsManagementScopeAndBoundedQueries(t *testing.T) {
	db, log := chatContextDB(t)
	counter := &treeQueryCounter{Interface: logger.Default.LogMode(logger.Silent)}
	db.DB = db.Session(&gorm.Session{Logger: counter})
	kbID, adminID, managerID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	navA, navB, folderA, folderB := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, item := range []any{
		&domain.KnowledgeBase{ID: kbID, Name: "learning-page-test"},
		&domain.User{ID: adminID, Account: "task_admin_" + adminID, Role: consts.UserRoleAdmin, Password: "unused"},
		&domain.User{ID: managerID, Account: "task_manager_" + managerID, Role: consts.UserRoleManager, Password: "unused"},
		&domain.Nav{ID: navA, KbID: kbID, Name: "managed", Permissions: domain.NodePermissions{ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: domain.ManagementSubjectUser, SubjectIDs: []string{managerID}, DirectoryScope: domain.ManagementScopeTree}}}},
		&domain.Nav{ID: navB, KbID: kbID, Name: "unmanaged"},
		&domain.Node{ID: folderA, KBID: kbID, NavId: navA, ParentID: navA, Name: "folder A", Type: domain.NodeTypeFolder},
		&domain.Node{ID: folderB, KBID: kbID, NavId: navB, ParentID: navB, Name: "folder B", Type: domain.NodeTypeFolder},
	} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	var nodes []domain.Node
	for i := 0; i < 2200; i++ {
		nav, parent := navA, folderA
		if i%2 == 1 {
			nav, parent = navB, folderB
		}
		nodes = append(nodes, domain.Node{ID: uuid.NewString(), KBID: kbID, NavId: nav, ParentID: parent, Name: fmt.Sprintf("doc-%04d", i), Type: domain.NodeTypeDocument, Status: domain.NodeStatusPublished, EditTime: time.Now(), RagInfo: domain.RagInfo{Status: consts.NodeRagStatusSucceeded}})
	}
	if err := db.CreateInBatches(nodes, 100).Error; err != nil {
		t.Fatal(err)
	}
	r := pg.NewNodeRepository(db, log)
	u := &NodeUsecase{nodeRepo: r, enterprise: &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(db, log)}}
	ctxFor := func(id string) context.Context {
		return context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: id})
	}
	for _, principal := range []struct {
		id    string
		count int64
	}{{adminID, 2200}, {managerID, 1100}} {
		ctx := ctxFor(principal.id)
		counter.count = 0
		start := time.Now()
		page, err := u.LearningTaskList(ctx, domain.LearningTaskQuery{KBID: kbID, Page: 2, PageSize: 10})
		if err != nil || page.Total != principal.count || len(page.List) != 10 {
			t.Fatalf("page failed %+v %v", page, err)
		}
		if counter.count > 12 {
			t.Fatalf("query count grew with documents: %d", counter.count)
		}
		raw, _ := json.Marshal(page)
		if len(raw) > 10000 {
			t.Fatalf("oversized 10-document payload: %d bytes", len(raw))
		}
		t.Logf("documents=%d queries=%d payload=%d bytes duration=%s", principal.count, counter.count, len(raw), time.Since(start))
		counts, err := u.LearningTaskSummary(ctx, kbID)
		if err != nil || counts.Total != principal.count {
			t.Fatalf("counts leaked %+v %v", counts, err)
		}
		ids, err := u.LearningTaskTargets(ctx, domain.LearningTaskQuery{KBID: kbID, PageSize: 10})
		if err != nil || int64(len(ids)) != principal.count {
			t.Fatalf("bulk targets are not all matches: %d %v", len(ids), err)
		}
		if principal.id == managerID {
			for _, doc := range page.List {
				if doc.NavID != navA {
					t.Fatal("unmanaged row leaked")
				}
			}
			if _, err := u.LearningTaskDetail(ctx, kbID, nodes[1].ID); err == nil {
				t.Fatal("unmanaged detail leaked")
			}
		}
	}
	if err := db.Model(&domain.Nav{}).Where("id = ?", navA).Update("permissions", domain.NodePermissions{}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := ctxFor(managerID)
	counts, err := u.LearningTaskSummary(ctx, kbID)
	if err != nil || counts.Total != 0 {
		t.Fatalf("revocation failed %+v %v", counts, err)
	}
	ids, err := u.LearningTaskTargets(ctx, domain.LearningTaskQuery{KBID: kbID})
	if err != nil || len(ids) != 0 {
		t.Fatalf("revoked targets leaked %v %v", ids, err)
	}
	if _, err := u.LearningTaskDetail(ctx, kbID, nodes[0].ID); err == nil {
		t.Fatal("revoked detail leaked")
	}
}
