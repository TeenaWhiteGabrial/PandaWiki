package usecase

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

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
	cfg := &config.Config{}
	l := log.NewLogger(cfg)
	storeDB := &store.DB{DB: tx}
	kbRepo := pg.NewKnowledgeBaseRepository(storeDB, cfg, l, &contextRAG{})
	u := &AppUsecase{repo: pg.NewAppRepository(storeDB, l), kbRepo: kbRepo}
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
	legacy, err := u.ShareGetWebAppInfo(context.Background(), kbID, "unverified-reader-id")
	if err != nil {
		t.Fatalf("anonymous legacy app info failed: %v", err)
	}
	legacyJSON, _ := json.Marshal(legacy)
	for _, secret := range []string{"private-document-id", "private-nav-id", "private-injected-code", "private-integration-secret"} {
		if strings.Contains(string(legacyJSON), secret) {
			t.Fatalf("anonymous legacy endpoint disclosed %s", secret)
		}
	}
	if legacy.Settings.AdminTitle != "Admin title" || len(legacy.Settings.WebAppLandingConfigs) != 0 || len(legacy.RecommendNodes) != 0 {
		t.Fatalf("legacy endpoint must return branding without expanding documents: %+v", legacy)
	}
	var unchanged domain.App
	if err := tx.First(&unchanged, "id = ?", app.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unchanged.Settings, app.Settings) {
		t.Fatal("reading public branding changed the application settings")
	}

	// A reader needs reading permission, not document-management permission.
	readerID, otherID := uuid.NewString(), uuid.NewString()
	for _, id := range []string{readerID, otherID} {
		if err := tx.Create(&domain.User{ID: id, Account: "app_reader_" + id, Password: "unused", Role: consts.UserRoleUser}).Error; err != nil {
			t.Fatal(err)
		}
	}
	public := domain.NodePermissions{ScopeType: "public"}
	restricted := domain.NodePermissions{ScopeType: "user", AllowedUserIDs: []string{otherID}}
	navID, hiddenNavID := uuid.NewString(), uuid.NewString()
	for _, nav := range []domain.Nav{
		{ID: navID, KbID: kbID, Name: "visible-nav", Permissions: public},
		{ID: hiddenNavID, KbID: kbID, Name: "hidden-nav-title", Permissions: restricted},
	} {
		if err := tx.Create(&nav).Error; err != nil {
			t.Fatal(err)
		}
	}
	releaseID := uuid.NewString()
	if err := tx.Create(&domain.KBRelease{ID: releaseID, KBID: kbID}).Error; err != nil {
		t.Fatal(err)
	}
	addNode := func(name, parentID, nav string, kind domain.NodeType, permissions domain.NodePermissions) string {
		t.Helper()
		id, versionID := uuid.NewString(), uuid.NewString()
		for _, fixture := range []any{
			&domain.Node{ID: id, KBID: kbID, Name: name, Type: kind, ParentID: parentID, NavId: nav, Permissions: permissions},
			&domain.NodeRelease{ID: versionID, NodeID: id, KBID: kbID, Name: name, Type: kind, ParentID: parentID},
			&domain.KBReleaseNodeRelease{ID: uuid.NewString(), KBID: kbID, NodeID: id, NodeReleaseID: versionID, ReleaseID: releaseID, NavID: nav},
		} {
			if err := tx.Create(fixture).Error; err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	visible := addNode("visible-document", navID, navID, domain.NodeTypeDocument, public)
	hidden := addNode("hidden-document", navID, navID, domain.NodeTypeDocument, restricted)
	hiddenByNav := addNode("hidden-by-nav-document", hiddenNavID, hiddenNavID, domain.NodeTypeDocument, public)
	folder := addNode("visible-folder", navID, navID, domain.NodeTypeFolder, public)
	addNode("visible-child", folder, navID, domain.NodeTypeDocument, public)
	hiddenChild := addNode("hidden-child", folder, navID, domain.NodeTypeDocument, restricted)
	allIDs := []string{visible, hidden, hiddenByNav, folder}
	app.Settings.RecommendNodeIDs = allIDs
	app.Settings.WebAppLandingConfigs = []domain.WebAppLandingConfig{
		{Type: "basic_doc", NodeIds: allIDs},
		{Type: "nav_doc", NavDocConfig: &domain.NavDocConfig{NavIds: []string{navID, hiddenNavID}}},
	}
	if err := tx.Model(app).Update("settings", app.Settings).Error; err != nil {
		t.Fatal(err)
	}
	nodes, navs := pg.NewNodeRepository(storeDB, l), pg.NewNavRepository(storeDB, l)
	enterprise := &EnterpriseUsecase{repo: pg.NewEnterpriseRepository(storeDB, l), nodeRepo: nodes, navRepo: navs}
	u.nodeUsecase = &NodeUsecase{nodeRepo: nodes, navRepo: navs, kbRepo: kbRepo, enterprise: enterprise}
	u.navRepo = navs
	ctx := context.WithValue(context.Background(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{UserId: readerID})
	info, err := u.ShareGetWebAppInfo(ctx, kbID, otherID)
	if err != nil {
		t.Fatalf("authenticated reader with no management grants failed: %v", err)
	}
	if !reflect.DeepEqual(info.Settings.RecommendNodeIDs, []string{visible, folder}) {
		t.Fatalf("reader recommendations ignored reading scope: %v", info.Settings.RecommendNodeIDs)
	}
	infoJSON, _ := json.Marshal(info)
	for _, blocked := range []string{hidden, hiddenByNav, hiddenChild, hiddenNavID, "hidden-document", "hidden-by-nav-document", "hidden-child", "hidden-nav-title"} {
		if strings.Contains(string(infoJSON), blocked) {
			t.Fatalf("reader recommendations disclosed unauthorized item %s", blocked)
		}
	}
	if !strings.Contains(string(infoJSON), "visible-child") {
		t.Fatal("readable child missing from recommended folder")
	}
	managed, manageErr := u.nodeUsecase.GetRecommendNodeList(ctx, &domain.GetRecommendNodeListReq{KBID: kbID, NodeIDs: allIDs})
	if manageErr == nil && len(managed) > 0 {
		t.Fatal("reader gained document-management recommendations")
	}

	// Branding URLs are stable application URLs; the public image resolver is
	// pinned to the selected field and cannot be used to sign arbitrary files.
	cfg.S3.Bucket = "pldms"
	cfg.S3.DownloadURL = "https://oss.example.test/pldms"
	u.config = cfg
	assetKey := kbID + "/assets/" + uuid.NewString() + ".ico"
	app.Settings.AdminFavicon = cfg.S3.DownloadURL + "/static-file/" + assetKey
	if err := tx.Model(app).Update("settings", app.Settings).Error; err != nil {
		t.Fatal(err)
	}
	brand, err = u.GetWebAppBrand(context.Background(), kbID)
	if err != nil || brand.Settings.AdminFavicon != "/share/v1/app/web/brand/image?field=admin_favicon" {
		t.Fatalf("private configured branding has no working application URL: %+v %v", brand, err)
	}
	if key, err := u.GetPublicBrandImageKey(context.Background(), kbID, "admin_favicon"); err != nil || key != assetKey {
		t.Fatalf("configured branding key cannot be resolved: %q %v", key, err)
	}
	if _, err := u.GetPublicBrandImageKey(context.Background(), kbID, "key"); err == nil {
		t.Fatal("arbitrary object signing field was accepted")
	}
	for _, value := range []string{
		"/api/v1/file/download?key=" + url.QueryEscape(hidden),
		cfg.S3.DownloadURL + "/static-file/" + kbID + "/private.pdf",
		"https://external.example.test/image.png",
	} {
		app.Settings.AdminFavicon = value
		if err := tx.Model(app).Update("settings", app.Settings).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := u.GetPublicBrandImageKey(context.Background(), kbID, "admin_favicon"); err == nil {
			t.Fatalf("non-display object became publicly signable: %q", value)
		}
	}
}
