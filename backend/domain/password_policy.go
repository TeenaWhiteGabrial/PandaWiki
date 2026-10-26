package domain

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrPasswordPolicy = errors.New(PasswordPolicyMessage)

const PasswordPolicyMessage = "密码需为 12～72 字节，包含大写字母、小写字母、数字和特殊字符，且不能包含账号"

// ValidatePassword is used by every account password write, not by login.
// The byte limit avoids bcrypt's silent truncation boundary.
func ValidatePassword(password, account string) error {
	if len(password) < 12 || len(password) > 72 || !utf8.ValidString(password) {
		return ErrPasswordPolicy
	}
	var upper, lower, digit, symbol bool
	for _, r := range password {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return ErrPasswordPolicy
		}
		switch {
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= '0' && r <= '9':
			digit = true
		default:
			symbol = true
		}
	}
	if !upper || !lower || !digit || !symbol {
		return ErrPasswordPolicy
	}
	if len(account) >= 3 && strings.Contains(strings.ToLower(password), strings.ToLower(account)) {
		return ErrPasswordPolicy
	}
	return nil
}
