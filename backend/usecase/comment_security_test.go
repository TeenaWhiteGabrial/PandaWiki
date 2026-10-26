package usecase

import (
	"context"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"testing"
)

func TestCommentIdentityAndKnowledgeBaseGuard(t *testing.T) {
	ctx := context.Background()
	if _, err := (&CommentUsecase{}).CreateComment(ctx, &domain.CommentReq{}, "kb", "127.0.0.1", domain.CommentStatusAccepted, ""); err == nil {
		t.Fatal("anonymous comment accepted")
	}
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN required for identity persistence check")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	defer tx.Rollback()
	userID, nodeID, kbID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	fixtures := []any{&domain.User{ID: userID, Account: "comment_test_" + userID, Name: "真实身份", Password: "unused", Role: consts.UserRoleUser}, &domain.KnowledgeBase{ID: kbID, Name: "temporary comment security"}, &domain.Node{ID: nodeID, KBID: kbID, Name: "temporary comment security", Type: domain.NodeTypeDocument}}
	for _, f := range fixtures {
		if err := tx.Create(f).Error; err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{}
	l := log.NewLogger(cfg)
	storeDB := &store.DB{DB: tx}
	u := NewCommentUsecase(pg.NewCommentRepository(storeDB, l), l, pg.NewNodeRepository(storeDB, l), nil, pg.NewUserRepository(storeDB, l))
	req := &domain.CommentReq{NodeID: nodeID, Content: "temporary test", UserName: "伪造管理员", PicUrls: []string{}}
	if _, err = u.CreateComment(ctx, req, uuid.NewString(), "127.0.0.1", domain.CommentStatusAccepted, userID); err == nil {
		t.Fatal("cross-KB comment accepted")
	}
	id, err := u.CreateComment(ctx, req, kbID, "127.0.0.1", domain.CommentStatusAccepted, userID)
	if err != nil {
		t.Fatal(err)
	}
	var row domain.Comment
	if err := tx.First(&row, "id=?", id).Error; err != nil {
		t.Fatal(err)
	}
	if row.Info.UserName != "真实身份" || row.Info.UserID != userID {
		t.Fatal("submitted display name was trusted")
	}
}
