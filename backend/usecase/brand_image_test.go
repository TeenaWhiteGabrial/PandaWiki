package usecase

import (
	"net/url"
	"testing"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
)

func TestPrivateBrandImageURLs(t *testing.T) {
	u := &AppUsecase{config: &config.Config{S3: config.S3Config{
		Bucket: "pldms", DownloadURL: "https://oss.example.test/pldms",
	}}}
	key := "1c55f96e-3791-4593-9288-197bf3344995/assets/fa12c276-4f06-437d-935d-a6e0332210f4.ico"
	canonical := "https://oss.example.test/pldms/static-file/" + key
	preview := "/api/v1/file/download?key=" + url.QueryEscape(key)
	for _, value := range []string{canonical, preview} {
		if got := u.adminBrandImageURL(value); got != preview {
			t.Fatalf("admin image must use an authenticated stable URL: %q", got)
		}
		settings := u.webAppBrandSettings(domain.AppSettings{Icon: value, AdminFavicon: value})
		if settings.Icon != "/share/v1/app/web/brand/image?field=icon" || settings.AdminFavicon != "/share/v1/app/web/brand/image?field=admin_favicon" {
			t.Fatalf("public branding retained a private OSS URL or an expiring signature: %+v", settings)
		}
		if got, ok := u.brandImageKey(value); !ok || got != key {
			t.Fatalf("configured branding key not preserved: %q %v", got, ok)
		}
	}
	for _, value := range []string{
		"https://attacker.test/pldms/static-file/" + key,
		"https://oss.example.test/pldms/static-file/" + key + "，",
		"https://oss.example.test/other/static-file/" + key,
		"https://oss.example.test/pldms/static-file/kb/document.pdf",
		"/api/v1/file/download?key=" + url.QueryEscape("kb/private-document.pdf"),
		"/api/v1/file/download?key=" + url.QueryEscape("kb/imports/archive/assets/icon.png"),
		"/api/v1/file/download?key=" + url.QueryEscape("../"+key),
	} {
		if _, ok := u.brandImageKey(value); ok {
			t.Fatalf("public branding could sign a non-display object: %q", value)
		}
	}
	if got := u.publicBrandImageURL("icon", "https://external.example.test/logo.png"); got != "https://external.example.test/logo.png" {
		t.Fatal("external branding URLs must remain unchanged")
	}
}
