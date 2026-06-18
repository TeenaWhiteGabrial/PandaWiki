package pg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

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
	return &APITokenRepo{
		db:     db,
		logger: logger,
		cache:  cache,
	}
}

func (r *APITokenRepo) GetByTokenWithCache(ctx context.Context, token string) (*domain.APIToken, error) {
	cacheKey := fmt.Sprintf("api_token:%s", token)

	cachedData, err := r.cache.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var apiToken domain.APIToken
		if err := json.Unmarshal([]byte(cachedData), &apiToken); err == nil {
			return &apiToken, nil
		}
	}

	// 缓存未命中，从数据库查询
	var apiToken domain.APIToken
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&apiToken).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get api token by token failed: %w", err)
	}

	if tokenData, err := json.Marshal(&apiToken); err == nil {
		if err := r.cache.Set(ctx, cacheKey, tokenData, 30*time.Minute).Err(); err != nil {
			r.logger.Warn("failed to cache API token", log.Error(err))
		}
	}

	return &apiToken, nil
}

func (r *APITokenRepo) Create(ctx context.Context, token *domain.APIToken) error {
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
