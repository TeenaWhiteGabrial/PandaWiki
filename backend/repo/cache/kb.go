package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/store/cache"
	"github.com/redis/go-redis/v9"
)

type KBRepo struct {
	cache *cache.Cache
}

func NewKBRepo(cache *cache.Cache) *KBRepo {
	return &KBRepo{cache: cache}
}

func (r *KBRepo) GetKB(ctx context.Context, kbID string) (*domain.KnowledgeBase, error) {
	kbStr, err := r.cache.Get(ctx, kbID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	if kbStr == "" {
		return nil, nil
	}

	var kb domain.KnowledgeBase
	err = json.Unmarshal([]byte(kbStr), &kb)
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (r *KBRepo) SetKB(ctx context.Context, kbID string, kb *domain.KnowledgeBase) error {
	kbStr, err := json.Marshal(kb)
	if err != nil {
		return err
	}
	return r.cache.Set(ctx, kbID, kbStr, 0).Err()
}

func (r *KBRepo) DeleteKB(ctx context.Context, kbID string) error {
	return r.cache.Del(ctx, kbID).Err()
}

func (r *KBRepo) ClearSession(ctx context.Context) error {
	return r.cache.DeleteKeysWithPrefix(ctx, "session_")
}

func publicationLockKey(kbID string) string {
	return "publication_lock:" + kbID
}

// AcquirePublicationLock prevents concurrent publication pipelines for one
// knowledge base across API instances. The TTL guarantees crash recovery.
func (r *KBRepo) AcquirePublicationLock(ctx context.Context, kbID, token string, ttl time.Duration) (bool, error) {
	return r.cache.SetNX(ctx, publicationLockKey(kbID), token, ttl).Result()
}

// ReleasePublicationLock only removes a lock still owned by token. This avoids
// deleting a newer lock after the original one has expired.
func (r *KBRepo) ReleasePublicationLock(ctx context.Context, kbID, token string) error {
	const releaseScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0`
	return r.cache.Eval(ctx, releaseScript, []string{publicationLockKey(kbID)}, token).Err()
}
