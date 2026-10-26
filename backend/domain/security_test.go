package domain

import (
	"strings"
	"testing"
)

func TestPasswordPolicyRejectsWeakAndTruncatedPasswords(t *testing.T) {
	for _, tc := range []struct {
		password, account string
		valid             bool
	}{
		{"11111111", "tester", false}, {"Abcd1234", "tester", false},
		{"abcdefgh123!", "tester", false}, {"ABCDEFGH123!", "tester", false},
		{"Abcdefghijkl!", "tester", false}, {"Abcdefgh1234", "tester", false},
		{"Tester@2026Strong", "tester", false}, {"Abcdefgh12! ", "tester", false},
		{strings.Repeat("a", 70) + "A1!", "tester", false},
		{"YunzhouWiki@2026", "pt_dev_g1", true}, {"Safe!Pass2026Go", "tester", true},
	} {
		if got := ValidatePassword(tc.password, tc.account) == nil; got != tc.valid {
			t.Errorf("password policy result=%v want=%v", got, tc.valid)
		}
	}
}

func TestAPITokenHashesCannotBeUsedAsCredentials(t *testing.T) {
	raw := strings.Repeat("a1", 24)
	hash := HashAPIToken(raw)
	if hash == raw || len(hash) != 71 || strings.Contains(hash, raw) {
		t.Fatal("token hash exposes credential or has invalid format")
	}
	if HashAPIToken(hash) == hash {
		t.Fatal("stored hash must not work as a bearer token")
	}
	if APITokenHint(raw) != raw[:6]+"…"+raw[len(raw)-4:] {
		t.Fatal("unexpected token hint")
	}
}
