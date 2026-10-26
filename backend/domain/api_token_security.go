package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashAPIToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func APITokenHint(raw string) string {
	if len(raw) < 12 {
		return "********"
	}
	return raw[:6] + "…" + raw[len(raw)-4:]
}
