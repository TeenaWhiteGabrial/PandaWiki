package usecase

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
)

func isDisplayAssetKey(key string) bool {
	parts := strings.Split(key, "/")
	if len(parts) != 3 || parts[1] != "assets" {
		return false
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		return false
	}
	ext := strings.ToLower(path.Ext(parts[2]))
	if mime := uploadTypes[ext]; !strings.HasPrefix(mime, "image/") {
		return false
	}
	_, err := uuid.Parse(strings.TrimSuffix(parts[2], path.Ext(parts[2])))
	return err == nil
}

// Accept only the active storage namespace or our relative admin download URL.
// A public branding endpoint must never sign arbitrary documents or remote URLs.
func (u *AppUsecase) brandImageKey(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	var key string
	if parsed.Scheme == "" && parsed.Host == "" && parsed.Path == "/api/v1/file/download" {
		key = parsed.Query().Get("key")
	} else {
		if u.config == nil {
			return "", false
		}
		base, err := url.Parse(strings.TrimRight(u.config.S3.DownloadURL, "/"))
		if err != nil || base.Host == "" || parsed.Scheme != base.Scheme || parsed.Host != base.Host || parsed.User != nil {
			return "", false
		}
		prefix := strings.TrimRight(base.Path, "/") + "/"
		if u.config.S3.Bucket != "" {
			prefix += "static-file/"
		}
		if !strings.HasPrefix(parsed.Path, prefix) {
			return "", false
		}
		key = strings.TrimPrefix(parsed.Path, prefix)
	}
	return key, isDisplayAssetKey(key)
}

func (u *AppUsecase) publicBrandImageURL(field, raw string) string {
	if _, ok := u.brandImageKey(raw); ok {
		return "/share/v1/app/web/brand/image?field=" + url.QueryEscape(field)
	}
	return raw
}

func (u *AppUsecase) adminBrandImageURL(raw string) string {
	if key, ok := u.brandImageKey(raw); ok {
		return "/api/v1/file/download?key=" + url.QueryEscape(key)
	}
	return raw
}

func (u *AppUsecase) webAppBrandSettings(settings domain.AppSettings) domain.AppBrandSettings {
	return domain.AppBrandSettings{
		Title: settings.Title, AdminTitle: settings.AdminTitle,
		Icon:         u.publicBrandImageURL("icon", settings.Icon),
		Favicon:      u.publicBrandImageURL("favicon", settings.Favicon),
		AdminLogo:    u.publicBrandImageURL("admin_logo", settings.AdminLogo),
		AdminFavicon: u.publicBrandImageURL("admin_favicon", settings.AdminFavicon),
	}
}

// Resolve a configured branding field, rather than accepting an object key.
// Only images already chosen as the site's public branding can be read here.
func (u *AppUsecase) GetPublicBrandImageKey(ctx context.Context, kbID, field string) (string, error) {
	if field != "icon" && field != "favicon" && field != "admin_logo" && field != "admin_favicon" {
		return "", fmt.Errorf("invalid branding image field")
	}
	app, err := u.repo.GetOrCreateAppByKBIDAndType(ctx, kbID, domain.AppTypeWeb)
	if err != nil {
		return "", err
	}
	values := map[string]string{
		"icon": app.Settings.Icon, "favicon": app.Settings.Favicon,
		"admin_logo": app.Settings.AdminLogo, "admin_favicon": app.Settings.AdminFavicon,
	}
	key, ok := u.brandImageKey(values[field])
	if !ok {
		return "", fmt.Errorf("configured branding image is not a validated display asset")
	}
	return key, nil
}
