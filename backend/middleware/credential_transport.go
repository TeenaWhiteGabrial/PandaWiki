package middleware

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/store/cache"
	"github.com/labstack/echo/v4"
)

const credentialLifetime = 2 * time.Minute
const credentialChallengeBytes = 24 // Unix timestamp (8) and random nonce (16).

type credentialKey struct {
	private *rsa.PrivateKey
	public  string
	signing [32]byte
}

type encryptedCredentials struct {
	Challenge  string `json:"challenge"`
	Key        string `json:"encrypted_key"`
	IV         string `json:"iv"`
	Ciphertext string `json:"ciphertext"`
}

func newCredentialKey() (*credentialKey, error) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	public, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
	if err != nil {
		return nil, err
	}
	key := &credentialKey{private: private, public: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}))}
	if _, err := rand.Read(key.signing[:]); err != nil {
		return nil, err
	}
	return key, nil
}

func (k *credentialKey) signature(challenge []byte) []byte {
	mac := hmac.New(sha256.New, k.signing[:])
	mac.Write(challenge)
	return mac.Sum(nil)
}

func (k *credentialKey) newChallenge() (string, error) {
	challenge := make([]byte, credentialChallengeBytes)
	binary.BigEndian.PutUint64(challenge, uint64(time.Now().Unix()))
	if _, err := rand.Read(challenge[8:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(append(challenge, k.signature(challenge)...)), nil
}

func (k *credentialKey) decrypt(body encryptedCredentials, aadPrefix string) []byte {
	challenge, err := base64.RawURLEncoding.Strict().DecodeString(body.Challenge)
	if err != nil || len(challenge) != credentialChallengeBytes+sha256.Size {
		return nil
	}
	age := time.Since(time.Unix(int64(binary.BigEndian.Uint64(challenge)), 0))
	if age < 0 || age >= credentialLifetime ||
		!hmac.Equal(challenge[credentialChallengeBytes:], k.signature(challenge[:credentialChallengeBytes])) {
		return nil
	}
	wrapped, err := base64.StdEncoding.DecodeString(body.Key)
	if err != nil || len(wrapped) != k.private.Size() {
		return nil
	}
	secret, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, k.private, wrapped, nil)
	if err != nil || len(secret) != 32 {
		return nil
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil
	}
	iv, err := base64.StdEncoding.DecodeString(body.IV)
	if err != nil || len(iv) != gcm.NonceSize() {
		return nil
	}
	ciphertext, err := base64.StdEncoding.DecodeString(body.Ciphertext)
	if err != nil {
		return nil
	}
	plaintext, err := gcm.Open(nil, iv, ciphertext, []byte(aadPrefix+body.Challenge))
	if err != nil {
		return nil
	}
	return plaintext
}

func credentialRoute(method, path string) bool {
	switch path {
	case "/api/v1/user/login", "/api/v1/user/create", "/api/v1/user/change_password",
		"/share/v1/auth/login/account", "/share/v1/auth/login/simple", "/share/v1/auth/change_password":
		return method == http.MethodPost
	case "/api/v1/user/reset_password":
		return method == http.MethodPut
	}
	return false
}

func RegisterEncryptedCredentials(e *echo.Echo, cache *cache.Cache) {
	loadKey := sync.OnceValues(newCredentialKey)
	reply := func(c echo.Context, status int, message string) error {
		return c.JSON(status, domain.PWResponse{Message: message})
	}
	challenge := func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		key, err := loadKey()
		if err != nil {
			return reply(c, http.StatusServiceUnavailable, "密码加密服务暂不可用")
		}
		value, err := key.newChallenge()
		if err != nil {
			return reply(c, http.StatusServiceUnavailable, "密码加密服务暂不可用")
		}
		return c.JSON(http.StatusOK, domain.PWResponse{Success: true, Data: map[string]string{
			"challenge": value, "public_key": key.public,
		}})
	}
	e.GET("/api/v1/user/credential-challenge", challenge)
	e.GET("/share/v1/auth/credential-challenge", challenge)
	e.Use(decryptCredentials(loadKey, func(ctx context.Context, nonce string) (bool, error) {
		return cache.SetNX(ctx, "credential_used:"+nonce, true, credentialLifetime).Result()
	}))
}

func decryptCredentials(loadKey func() (*credentialKey, error), claim func(context.Context, string) (bool, error)) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			if !credentialRoute(r.Method, c.Path()) {
				return next(c)
			}
			c.Response().Header().Set("Cache-Control", "no-store")
			reject := func() error {
				return c.JSON(http.StatusBadRequest, domain.PWResponse{Message: "密码加密请求无效、已使用或已过期，请重新提交"})
			}
			var body encryptedCredentials
			limited := http.MaxBytesReader(c.Response(), r.Body, 64<<10)
			decoder := json.NewDecoder(limited)
			decoder.DisallowUnknownFields()
			err := decoder.Decode(&body)
			valid := err == nil && decoder.Decode(&struct{}{}) == io.EOF
			limited.Close()
			if !valid || body.Challenge == "" {
				return reject()
			}
			key, err := loadKey()
			if err != nil {
				return c.JSON(http.StatusServiceUnavailable, domain.PWResponse{Message: "密码加密服务暂不可用"})
			}
			plaintext := key.decrypt(body, r.Method+" "+c.Path()+"\n")
			if plaintext == nil {
				return reject()
			}
			accepted, err := claim(r.Context(), body.Challenge)
			if err != nil {
				return c.JSON(http.StatusServiceUnavailable, domain.PWResponse{Message: "密码加密服务暂不可用"})
			}
			if !accepted {
				return reject()
			}
			r.Body = io.NopCloser(bytes.NewReader(plaintext))
			r.ContentLength = int64(len(plaintext))
			r.Header.Set("Content-Length", strconv.Itoa(len(plaintext)))
			r.Header.Set("Content-Type", "application/json")
			return next(c)
		}
	}
}
