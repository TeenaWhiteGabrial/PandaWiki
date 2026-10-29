package usecase

import (
	"testing"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/google/uuid"
)

func TestCancelPendingLearningRejectsNonSuperAdmin(t *testing.T) {
	db, l := chatContextDB(t)
	for _, role := range []consts.UserRole{consts.UserRoleManager, consts.UserRoleUser} {
		user := domain.User{ID: uuid.NewString(), Account: uuid.NewString(), Role: role}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		u := &NodeUsecase{userRepo: pg.NewUserRepository(db, l)}
		if _, err := u.CancelPendingLearning(t.Context(), uuid.NewString(), user.ID); err == nil {
			t.Fatal("non-super-admin could cancel all knowledge-base submissions")
		}
	}
}
