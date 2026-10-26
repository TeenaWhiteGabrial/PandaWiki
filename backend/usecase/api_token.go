package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	tokenV1 "github.com/chaitin/panda-wiki/api/token/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
)

type APITokenUsecase struct {
	repo      *pg.APITokenRepo
	kbUsecase *KnowledgeBaseUsecase
	logger    *log.Logger
}

func NewAPITokenUsecase(
	repo *pg.APITokenRepo,
	kbUsecase *KnowledgeBaseUsecase,
	logger *log.Logger,
) *APITokenUsecase {
	return &APITokenUsecase{
		repo:      repo,
		kbUsecase: kbUsecase,
		logger:    logger.WithModule("usecase.api_token"),
	}
}

func (u *APITokenUsecase) CreateMCPToken(ctx context.Context, userID string, req *tokenV1.CreateAPITokenReq) (*tokenV1.APITokenListItem, error) {
	if req.Permission != consts.UserKBPermissionNotNull && req.Permission != consts.UserKBPermissionDocManage {
		return nil, fmt.Errorf("mcp token permission must be not null or doc_manage")
	}
	if _, err := u.kbUsecase.GetKnowledgeBase(ctx, req.KbID); err != nil {
		return nil, err
	}
	userPerm, err := u.kbUsecase.GetKnowledgeBasePerm(ctx, req.KbID)
	if err != nil {
		return nil, err
	}
	if !isAllowedTokenPermission(req.Permission, userPerm) {
		return nil, fmt.Errorf("permission exceeds your knowledge base access")
	}
	tokenValue, err := u.repo.BuildToken()
	if err != nil {
		return nil, err
	}
	model := &domain.APIToken{
		ID:         uuid.New().String(),
		Name:       req.Name,
		UserID:     userID,
		Token:      tokenValue,
		KbId:       req.KbID,
		Permission: req.Permission,
	}
	if err := u.repo.Create(ctx, model); err != nil {
		return nil, err
	}
	result := toAPITokenListItem(model)
	result.Token = tokenValue // Only this create response can reveal the raw credential.
	return result, nil
}

func (u *APITokenUsecase) ListMCPToken(ctx context.Context, userID, kbID string) ([]tokenV1.APITokenListItem, error) {
	if _, err := u.kbUsecase.GetKnowledgeBase(ctx, kbID); err != nil {
		return nil, err
	}
	tokens, err := u.repo.ListByUserAndKB(ctx, userID, kbID)
	if err != nil {
		return nil, err
	}
	resp := make([]tokenV1.APITokenListItem, 0, len(tokens))
	for i := range tokens {
		resp = append(resp, *toAPITokenListItem(&tokens[i]))
	}
	return resp, nil
}

func (u *APITokenUsecase) DeleteMCPToken(ctx context.Context, userID, kbID, id string) error {
	if _, err := u.kbUsecase.GetKnowledgeBase(ctx, kbID); err != nil {
		return err
	}
	return u.repo.DeleteByID(ctx, id, userID, kbID)
}

func toAPITokenListItem(token *domain.APIToken) *tokenV1.APITokenListItem {
	if token == nil {
		return nil
	}
	return &tokenV1.APITokenListItem{
		ID:         token.ID,
		Name:       token.Name,
		TokenHint:  token.TokenHint,
		Permission: token.Permission,
		CreatedAt:  token.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  token.UpdatedAt.Format(time.RFC3339),
	}
}

func isAllowedTokenPermission(requested, actual consts.UserKBPermission) bool {
	switch actual {
	case consts.UserKBPermissionFullControl:
		return requested == consts.UserKBPermissionDocManage || requested == consts.UserKBPermissionNotNull
	case consts.UserKBPermissionDocManage:
		return requested == consts.UserKBPermissionDocManage || requested == consts.UserKBPermissionNotNull
	case consts.UserKBPermissionNotNull:
		return requested == consts.UserKBPermissionNotNull
	default:
		return false
	}
}
