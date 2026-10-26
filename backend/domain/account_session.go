package domain

import "time"

const SessionInvalidCode = 40101
const PasswordChangeRequiredCode = 40302

type AccountSession struct {
	ID         string `gorm:"primaryKey"`
	UserID     string
	ExpiresAt  time.Time
	LastSeenAt time.Time
	CreatedAt  time.Time
}

type AccountSessionState struct {
	PasswordChangeRequired bool
}
