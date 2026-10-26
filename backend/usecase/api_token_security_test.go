package usecase

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaitin/panda-wiki/domain"
)

func TestTokenListNeverContainsStoredOrRawCredential(t *testing.T) {
	raw := strings.Repeat("b7", 24)
	token := &domain.APIToken{ID: "test", Token: domain.HashAPIToken(raw), TokenHint: domain.APITokenHint(raw)}
	item := toAPITokenListItem(token)
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if item.Token != "" || strings.Contains(string(data), raw) || strings.Contains(string(data), token.Token) {
		t.Fatal("list response contains a reusable credential or hash")
	}
	if item.TokenHint != token.TokenHint {
		t.Fatal("masked hint missing")
	}
}
