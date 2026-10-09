package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

func TestConversationHistoryIsolationAndResumePostgres(t *testing.T) {
	db, logger := chatContextDB(t)
	repo := pg.NewConversationRepository(db, logger)
	u := &ConversationUsecase{repo: repo}
	ctx := context.Background()
	kbID := uuid.NewString()
	owner := seedChatHistory(t, db, kbID)
	second := seedChatHistory(t, db, kbID)
	otherUser := seedChatHistory(t, db, kbID)
	otherKB := seedChatHistory(t, db, uuid.NewString())
	anon := seedChatHistory(t, db, kbID)
	anotherAnon := seedChatHistory(t, db, kbID)
	for _, conversation := range []*domain.Conversation{second, otherKB, anon, anotherAnon} {
		userID := owner.Info.UserInfo.UserID
		if conversation == anon || conversation == anotherAnon {
			userID = ""
		}
		conversation.Info = domain.ConversationInfo{UserInfo: domain.UserInfo{UserID: userID}}
		if err := db.Model(conversation).Update("info", conversation.Info).Error; err != nil {
			t.Fatal(err)
		}
	}
	latest := time.Now().Truncate(time.Microsecond).Add(time.Minute).UTC()
	if err := repo.CreateConversationMessage(ctx, &domain.ConversationMessage{ID: uuid.NewString(), KBID: kbID, AppID: owner.AppID, ConversationID: owner.ID, Role: schema.User, Content: "解释第二条", CreatedAt: latest}, nil); err != nil {
		t.Fatal(err)
	}
	credential := func(conversation *domain.Conversation) domain.ConversationHistoryCredential {
		return domain.ConversationHistoryCredential{ID: conversation.ID, Nonce: conversation.Nonce}
	}
	for _, test := range []struct {
		name  string
		user  string
		creds []domain.ConversationHistoryCredential
		want  int
	}{
		{"owner sees only own KB conversations", owner.Info.UserInfo.UserID, []domain.ConversationHistoryCredential{credential(otherUser), credential(anon)}, 2},
		{"anonymous without credentials sees nothing", "", nil, 0},
		{"anonymous cannot enumerate by ID", "", []domain.ConversationHistoryCredential{{ID: anon.ID, Nonce: "wrong"}}, 0},
		{"anonymous credential opens only matching chat", "", []domain.ConversationHistoryCredential{credential(anon)}, 1},
		{"logged-out browser cannot list account conversations", "", []domain.ConversationHistoryCredential{credential(owner)}, 0},
		{"cross KB credentials cannot leak history", "", []domain.ConversationHistoryCredential{credential(otherKB)}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := u.GetShareConversationHistory(ctx, kbID, test.user, &domain.ShareConversationHistoryRequest{Page: 1, PageSize: 20, Sessions: test.creds})
			if err != nil || result.Total != uint64(test.want) || len(result.Data) != test.want {
				t.Fatalf("unexpected history: %#v %v", result, err)
			}
		})
	}
	page, err := u.GetShareConversationHistory(ctx, kbID, owner.Info.UserInfo.UserID, &domain.ShareConversationHistoryRequest{Page: 1, PageSize: 1})
	if err != nil || page.Total != 2 || len(page.Data) != 1 || page.Data[0].ID != owner.ID || !page.Data[0].UpdatedAt.Equal(latest.Truncate(time.Microsecond)) {
		t.Fatalf("latest followup did not sort first: %#v %v", page, err)
	}
	next, err := u.GetShareConversationHistory(ctx, kbID, owner.Info.UserInfo.UserID, &domain.ShareConversationHistoryRequest{Page: 2, PageSize: 1})
	if err != nil || len(next.Data) != 1 || next.Data[0].ID != second.ID {
		t.Fatalf("pagination lost history: %#v %v", next, err)
	}
	if err := db.Model(second).Update("subject", "安全_%记录").Error; err != nil {
		t.Fatal(err)
	}
	filtered, err := u.GetShareConversationHistory(ctx, kbID, owner.Info.UserInfo.UserID, &domain.ShareConversationHistoryRequest{Page: 1, PageSize: 20, Query: "_%"})
	if err != nil || filtered.Total != 1 || filtered.Data[0].ID != second.ID {
		t.Fatalf("literal title search failed: %#v %v", filtered, err)
	}
	session, err := u.ResumeShareConversation(ctx, kbID, page.Data[0].ID, "", owner.Info.UserInfo.UserID, "test")
	if err != nil || session.Forked || session.ConversationID != owner.ID {
		t.Fatalf("history selection must continue original session: %#v %v", session, err)
	}
}
