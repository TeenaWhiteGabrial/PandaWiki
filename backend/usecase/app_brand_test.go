package usecase

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPublicAppBrandPostgres(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN for PostgreSQL integration")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	kbID := uuid.NewString()
	if err := tx.Create(&domain.KnowledgeBase{ID: kbID, Name: "private knowledge base"}).Error; err != nil {
		t.Fatal(err)
	}
	app := &domain.App{ID: uuid.NewString(), KBID: kbID, Type: domain.AppTypeWeb, Settings: domain.AppSettings{
		Title: "Reader title", Icon: "/reader.png", Favicon: "/reader.ico",
		AdminTitle: "Admin title", AdminLogo: "/admin.png", AdminFavicon: "/admin.ico",
		HeadCode: "private-injected-code", FeishuBotAppSecret: "private-integration-secret",
		RecommendNodeIDs: []string{"private-document-id"},
		WebAppLandingConfigs: []domain.WebAppLandingConfig{
			{Type: "basic_doc", NodeIds: []string{"private-document-id"}},
			{Type: "nav_doc", NavDocConfig: &domain.NavDocConfig{NavIds: []string{"private-nav-id"}}},
		},
	}}
	if err := tx.Create(app).Error; err != nil {
		t.Fatal(err)
	}
	// No identity, node usecase, permission service or cache: branding must not
	// expand either document recommendations or navigation recommendations.
	u := &AppUsecase{repo: pg.NewAppRepository(&store.DB{DB: tx}, log.NewLogger(&config.Config{}))}
	brand, err := u.GetWebAppBrand(context.Background(), kbID)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(brand)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"settings": map[string]any{
		"title": "Reader title", "icon": "/reader.png", "favicon": "/reader.ico",
		"admin_title": "Admin title", "admin_logo": "/admin.png", "admin_favicon": "/admin.ico",
	}}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("public branding included non-brand fields or lost branding: %s", encoded)
	}
	var unchanged domain.App
	if err := tx.First(&unchanged, "id = ?", app.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unchanged.Settings, app.Settings) {
		t.Fatal("reading public branding changed the application settings")
	}
}
