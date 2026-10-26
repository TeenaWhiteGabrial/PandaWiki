package pg

import (
	"context"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"testing"
	"time"
)

func securityTestDB(t *testing.T) *store.DB {
	t.Helper()
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback(); conn.Close() })
	return &store.DB{DB: tx}
}
func TestAccountSessionAndPasswordRevocation(t *testing.T) {
	db := securityTestDB(t)
	ctx := context.Background()
	r := &UserRepository{db: db}
	sessions := r.AccountSessions()
	hash, _ := bcrypt.GenerateFromPassword([]byte("Initial_Test#794"), bcrypt.MinCost)
	user := domain.User{ID: uuid.NewString(), Account: "session_test_" + uuid.NewString(), Password: string(hash), Role: consts.UserRoleUser, PasswordChangeRequired: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	makeSession := func() string {
		id := uuid.NewString()
		if err := sessions.Create(ctx, id, user.ID, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		return id
	}
	assertInvalid := func(id string) {
		state, err := sessions.Validate(ctx, id, user.ID, time.Now(), 30*time.Minute)
		if err != nil || state != nil {
			t.Fatalf("revoked/expired session accepted: %+v %v", state, err)
		}
	}
	id := makeSession()
	state, err := sessions.Validate(ctx, id, user.ID, now, 30*time.Minute)
	if err != nil || state == nil || !state.PasswordChangeRequired {
		t.Fatal(state, err)
	}
	if err := sessions.Revoke(ctx, id, user.ID); err != nil {
		t.Fatal(err)
	}
	assertInvalid(id)
	id = makeSession()
	db.Model(&domain.AccountSession{}).Where("id=?", id).Update("last_seen_at", now.Add(-31*time.Minute))
	assertInvalid(id)
	id = makeSession()
	db.Model(&domain.AccountSession{}).Where("id=?", id).Update("expires_at", now.Add(-time.Second))
	assertInvalid(id)
	id = makeSession()
	if err := r.ChangeOwnPassword(ctx, user.ID, "Changed_Strong#795"); err != nil {
		t.Fatal(err)
	}
	assertInvalid(id)
	var updated domain.User
	db.First(&updated, "id=?", user.ID)
	if updated.PasswordChangeRequired || bcrypt.CompareHashAndPassword([]byte(updated.Password), []byte("Changed_Strong#795")) != nil {
		t.Fatal("own change did not rotate password and clear flag")
	}
	id = makeSession()
	if err := r.UpdateUserPassword(ctx, user.ID, "Reset_Strong#796"); err != nil {
		t.Fatal(err)
	}
	assertInvalid(id)
	db.First(&updated, "id=?", user.ID)
	if !updated.PasswordChangeRequired {
		t.Fatal("reset must force a new password")
	}
}
func TestAPITokenDatabaseHashMigrationAndDeletion(t *testing.T) {
	db := securityTestDB(t)
	ctx := context.Background()
	r := &APITokenRepo{db: db}
	raw := "dedicated_security_test_key_7e2a9bc55841"
	item := &domain.APIToken{ID: uuid.NewString(), UserID: uuid.NewString(), KbId: uuid.NewString(), Name: "test", Token: raw, Permission: consts.UserKBPermissionFullControl}
	if err := r.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if item.Token != domain.HashAPIToken(raw) || item.TokenHint == "" {
		t.Fatal("plaintext stored")
	}
	got, err := r.GetByTokenWithCache(ctx, raw)
	if err != nil || got == nil {
		t.Fatal("raw key cannot authenticate", err)
	}
	got, err = r.GetByTokenWithCache(ctx, item.Token)
	if err != nil || got != nil {
		t.Fatal("stored hash is reusable", err)
	}
	if err := r.DeleteByID(ctx, item.ID, item.UserID, item.KbId); err != nil {
		t.Fatal(err)
	}
	got, err = r.GetByTokenWithCache(ctx, raw)
	if err != nil || got != nil {
		t.Fatal("deleted key still accepted", err)
	}
	old := domain.APIToken{ID: uuid.NewString(), UserID: uuid.NewString(), KbId: uuid.NewString(), Name: "legacy", Token: raw + "old", Permission: consts.UserKBPermissionFullControl}
	if err := db.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE api_tokens SET token_hint=left(token,6)||'…'||right(token,4),token='sha256:'||encode(sha256(convert_to(token,'UTF8')),'hex') WHERE id=?`, old.ID).Error; err != nil {
		t.Fatal(err)
	}
	got, err = r.GetByTokenWithCache(ctx, old.Token)
	if err != nil || got == nil || got.Token != domain.HashAPIToken(old.Token) {
		t.Fatal("legacy migration invalidates key", err)
	}
}

func TestBootstrapPasswordDoesNotOverwriteChangedPassword(t *testing.T) {
	db := securityTestDB(t)
	r := &UserRepository{db: db}
	ctx := context.Background()
	user := domain.User{ID: uuid.NewString(), Account: "bootstrap_" + uuid.NewString(), Name: "initial", Password: "Initial_Strong#794", Role: consts.UserRoleAdmin}
	if err := r.UpsertDefaultUser(ctx, &user); err != nil {
		t.Fatal(err)
	}
	var first domain.User
	if err := db.First(&first, "id=?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !first.PasswordChangeRequired {
		t.Fatal("initial administrator must change the password")
	}
	if err := r.ChangeOwnPassword(ctx, user.ID, "Changed_Strong#795"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertDefaultUser(ctx, &domain.User{ID: uuid.NewString(), Account: user.Account, Password: "old-env-password", Role: consts.UserRoleAdmin}); err != nil {
		t.Fatal(err)
	}
	var after domain.User
	db.First(&after, "id=?", user.ID)
	if after.PasswordChangeRequired || bcrypt.CompareHashAndPassword([]byte(after.Password), []byte("Changed_Strong#795")) != nil {
		t.Fatal("restart overwrote changed password or forced another change")
	}
}
