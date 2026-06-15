package v1

import "github.com/chaitin/panda-wiki/consts"

type AuthLoginSimpleReq struct {
	Password string `json:"password" validate:"required"`
}

type AuthLoginSimpleResp struct {
}

type AuthGetReq struct {
}
type AuthGetResp struct {
	AuthType       consts.AuthType       `json:"auth_type"`
	SourceType     consts.SourceType     `json:"source_type"`
	LicenseEdition consts.LicenseEdition `json:"license_edition"`
}

// 保留该类型仅用于兼容仍参与编译的 GitHub OAuth usecase。
// 分享端运行路由已移除，不再对外暴露该登录入口。
type AuthGitHubReq struct {
	KbID        string `json:"kb_id" validate:"required"`
	RedirectUrl string `json:"redirect_url" validate:"required"`
}

// 保留该类型仅用于兼容仍参与编译的 GitHub OAuth usecase。
// 分享端运行路由已移除，不再对外暴露该回调入口。
type GitHubCallbackReq struct {
	Code  string `query:"code" validate:"required"`
	State string `query:"state" validate:"required"`
}
