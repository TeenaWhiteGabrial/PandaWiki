package middleware

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestEncryptedCredentialBoundary(t *testing.T) {
	key, err := newCredentialKey()
	if err != nil {
		t.Fatal(err)
	}
	const path = "/api/v1/user/reset_password"
	plaintext := []byte(`{"password":"合成密码-Example42!"}`)
	challenge, err := key.newChallenge()
	if err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{7}, 32)
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &key.private.PublicKey, secret, nil)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	iv := make([]byte, gcm.NonceSize())
	encode := base64.StdEncoding.EncodeToString
	envelope, err := json.Marshal(encryptedCredentials{
		Challenge: challenge, Key: encode(wrapped), IV: encode(iv),
		Ciphertext: encode(gcm.Seal(nil, iv, plaintext, []byte("PUT "+path+"\n"+challenge))),
	})
	if err != nil {
		t.Fatal(err)
	}
	used := false
	e := echo.New()
	e.Use(decryptCredentials(func() (*credentialKey, error) { return key, nil }, func(context.Context, string) (bool, error) {
		allowed := !used
		used = true
		return allowed, nil
	}))
	calls := 0
	verify := func(c echo.Context) error {
		calls++
		body, err := io.ReadAll(c.Request().Body)
		if err != nil || !bytes.Equal(body, plaintext) {
			t.Fatal("handler did not receive the original request")
		}
		return c.NoContent(http.StatusNoContent)
	}
	e.PUT(path, verify)
	e.POST("/api/v1/user/change_password", verify)
	e.Group("share/v1/auth").POST("/login/simple", verify)

	for _, tc := range []struct {
		method, path string
		body         []byte
		status       int
	}{
		{"PUT", path, plaintext, http.StatusBadRequest},
		{"POST", "/share/v1/auth/login/simple", plaintext, http.StatusBadRequest},
		{"POST", "/api/v1/user/change_password", envelope, http.StatusBadRequest},
		{"PUT", path, envelope, http.StatusNoContent},
		{"PUT", path, envelope, http.StatusBadRequest},
	} {
		response := httptest.NewRecorder()
		e.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body)))
		if response.Code != tc.status {
			t.Fatalf("%s %s status=%d want=%d", tc.method, tc.path, response.Code, tc.status)
		}
	}
	if calls != 1 {
		t.Fatalf("credential handler called %d times", calls)
	}
}
