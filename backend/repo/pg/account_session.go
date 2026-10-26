package pg

import (
	"context"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	store "github.com/chaitin/panda-wiki/store/pg"
)

type AccountSessionRepository struct{ db *store.DB }

func (r *UserRepository) AccountSessions() *AccountSessionRepository {
	return &AccountSessionRepository{db: r.db}
}
func (r *UserAccessRepository) AccountSessions() *AccountSessionRepository {
	return &AccountSessionRepository{db: r.db}
}
func (r *APITokenRepo) AccountSessions() *AccountSessionRepository {
	return &AccountSessionRepository{db: r.db}
}

func (r *AccountSessionRepository) Create(ctx context.Context, id, userID string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).Create(&domain.AccountSession{ID: id, UserID: userID, ExpiresAt: expiresAt, LastSeenAt: time.Now()}).Error
}

// The conditional update atomically checks and touches a live session. A deleted,
// expired or idle session cannot be brought back to life by a later request.
func (r *AccountSessionRepository) Validate(ctx context.Context, id, userID string, now time.Time, idle time.Duration) (*domain.AccountSessionState, error) {
	var state domain.AccountSessionState
	result := r.db.WithContext(ctx).Raw(`WITH live AS (
 UPDATE account_sessions SET last_seen_at = ?
 WHERE id = ? AND user_id = ? AND expires_at > ? AND last_seen_at > ?
 RETURNING user_id)
 SELECT users.password_change_required FROM users JOIN live ON users.id = live.user_id`, now, id, userID, now, now.Add(-idle)).Scan(&state)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, nil
	}
	return &state, nil
}
func (r *AccountSessionRepository) Revoke(ctx context.Context, id, userID string) error {
	return r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&domain.AccountSession{}).Error
}
func (r *AccountSessionRepository) RevokeUser(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.AccountSession{}).Error
}
