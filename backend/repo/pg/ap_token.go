package pg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"gorm.io/gorm"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/cache"
	"github.com/chaitin/panda-wiki/store/pg"
)

type APITokenRepo struct {
	db     *pg.DB
	logger *log.Logger
	cache  *cache.Cache
}

func NewAPITokenRepo(db *pg.DB, logger *log.Logger, cache *cache.Cache) *APITokenRepo {
	// Remove the old plaintext credential cache namespace during rollout.
	if cache != nil {
		if err := cache.DeleteKeysWithPrefix(context.Background(), "api_token:"); err != nil {
			logger.Warn("clear legacy API token cache failed", log.Error(err))
		}
	}
	return &APITokenRepo{
		db:     db,
		logger: logger,
		cache:  cache,
	}
}

// Kept under its historical name for callers. Authentication always checks the
// database so deletion is immediately effective and credentials never enter Redis.
func (r *APITokenRepo) GetByTokenWithCache(ctx context.Context, token string) (*domain.APIToken, error) {
	var apiToken domain.APIToken
	if err := r.db.WithContext(ctx).Where("token = ?", domain.HashAPIToken(token)).First(&apiToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get api token failed: %w", err)
	}
	return &apiToken, nil
}

func (r *APITokenRepo) Create(ctx context.Context, token *domain.APIToken) error {
	token.TokenHint = domain.APITokenHint(token.Token)
	token.Token = domain.HashAPIToken(token.Token)
	if err := r.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("create api token failed: %w", err)
	}
	return nil
}

func (r *APITokenRepo) ListByUserAndKB(ctx context.Context, userID, kbID string) ([]domain.APIToken, error) {
	var tokens []domain.APIToken
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND kb_id = ?", userID, kbID).
		Order("created_at desc").
		Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("list api tokens failed: %w", err)
	}
	return tokens, nil
}

func (r *APITokenRepo) DeleteByID(ctx context.Context, id, userID, kbID string) error {
	if err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND kb_id = ?", id, userID, kbID).
		Delete(&domain.APIToken{}).Error; err != nil {
		return fmt.Errorf("delete api token failed: %w", err)
	}
	return nil
}

func (r *APITokenRepo) BuildToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token failed: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
