package v1

import "github.com/chaitin/panda-wiki/consts"

type CreateAPITokenReq struct {
	KbID       string                  `json:"kb_id" validate:"required"`
	Name       string                  `json:"name" validate:"required"`
	Permission consts.UserKBPermission `json:"permission" validate:"required,oneof=doc_manage"`
}

type APITokenListReq struct {
	KbID string `json:"kb_id" query:"kb_id" validate:"required"`
}

type APITokenListItem struct {
	ID         string                  `json:"id"`
	Name       string                  `json:"name"`
	Token      string                  `json:"token"`
	Permission consts.UserKBPermission `json:"permission"`
	CreatedAt  string                  `json:"created_at"`
	UpdatedAt  string                  `json:"updated_at"`
}

type DeleteAPITokenReq struct {
	ID   string `json:"id" query:"id" validate:"required"`
	KbID string `json:"kb_id" query:"kb_id" validate:"required"`
}
