package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/usecase"
)

type ShareAuthMiddleware struct {
	config       *config.Config
	logger       *log.Logger
	kbUsecase    *usecase.KnowledgeBaseUsecase
	apiTokenRepo *pg.APITokenRepo
}

func NewShareAuthMiddleware(config *config.Config, logger *log.Logger, kbUsecase *usecase.KnowledgeBaseUsecase, apiTokenRepo *pg.APITokenRepo) *ShareAuthMiddleware {
	return &ShareAuthMiddleware{
		config:       config,
		logger:       logger.WithModule("middleware.share_auth"),
		kbUsecase:    kbUsecase,
		apiTokenRepo: apiTokenRepo,
	}
}

func (h *ShareAuthMiddleware) applyJWTAuth(c echo.Context, kbID string) bool {
	authHeader := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" || !strings.Contains(tokenString, ".") {
		return false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return []byte(h.config.Auth.JWT.Secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || token == nil || !token.Valid {
		h.logger.Warn("share jwt auth failed", log.Error(err))
		return false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}

	userID, ok := claims["id"].(string)
	if !ok || userID == "" {
		return false
	}

	if h.apiTokenRepo == nil {
		return false
	}
	sessionID, state, sessionErr := checkAccountSession(c, claims, h.apiTokenRepo.AccountSessions(), h.config)
	if sessionErr != nil {
		return false
	}
	ctx := context.WithValue(c.Request().Context(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{
		SessionExpiresAt:       accountSessionExpiresAt(claims),
		SessionID:              sessionID,
		PasswordChangeRequired: state.PasswordChangeRequired,
		IsToken:                false,
		Permission:             consts.UserKBPermissionDocManage,
		UserId:                 userID,
		KBId:                   kbID,
	})
	c.SetRequest(c.Request().WithContext(ctx))
	return true
}

func (h *ShareAuthMiddleware) resolveKBID(ctx context.Context, kbID string) (string, error) {
	if kbID != "" {
		return kbID, nil
	}
	kb, err := h.kbUsecase.GetDefaultKnowledgeBase(ctx)
	if err != nil {
		return "", err
	}
	return kb.ID, nil
}

func (h *ShareAuthMiddleware) CheckForbidden(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		kbID, err := h.resolveKBID(c.Request().Context(), c.Request().Header.Get("X-KB-ID"))
		if err != nil {
			h.logger.Error("kb_id resolve failed", log.Error(err))
			return c.JSON(http.StatusBadRequest, domain.PWResponse{
				Success: false,
				Message: "kb_id is required",
			})
		}
		c.Request().Header.Set("X-KB-ID", kbID)

		kb, err := h.kbUsecase.GetKnowledgeBase(c.Request().Context(), kbID)
		if err != nil {
			h.logger.Error("get knowledge base failed", log.String("kb_id", kbID), log.Error(err))
			sentry.CaptureException(err)
			return c.JSON(http.StatusInternalServerError, domain.PWResponse{
				Success: false,
				Message: "failed to get knowledge base detail",
			})
		}

		if kb.AccessSettings.IsForbidden {
			h.logger.Warn("access forbidden", log.String("kb_id", kbID))
			return c.JSON(http.StatusForbidden, domain.PWResponse{
				Success: false,
				Message: "access is forbidden",
			})
		}

		return next(c)
	}
}

// OptionalAuthorize allows anonymous page metadata, but validates any supplied
// reader credential before the handler can return identity-dependent content.
func (h *ShareAuthMiddleware) OptionalAuthorize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "private, no-store")
		if strings.TrimSpace(c.Request().Header.Get("Authorization")) == "" {
			return next(c)
		}
		return h.Authorize(next)(c)
	}
}

func (h *ShareAuthMiddleware) Authorize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// A public document means all authenticated readers, not anonymous
		// visitors. Reject missing credentials before resolving any KB data.
		c.Response().Header().Set("Cache-Control", "private, no-store")
		authHeader := strings.TrimSpace(c.Request().Header.Get("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")) == "" {
			return c.JSON(http.StatusUnauthorized, domain.PWResponse{
				Success: false, Code: domain.SessionInvalidCode, Message: "请先登录后访问知识库",
			})
		}
		c.Request().Header.Set("Authorization", authHeader)
		kbID, err := h.resolveKBID(c.Request().Context(), c.Request().Header.Get("X-KB-ID"))
		if err != nil {
			h.logger.Error("kb_id resolve failed", log.Error(err))
			return c.JSON(http.StatusUnauthorized, domain.PWResponse{
				Success: false,
				Message: "Unauthorized",
			})
		}
		c.Request().Header.Set("X-KB-ID", kbID)

		kb, err := h.kbUsecase.GetKnowledgeBase(c.Request().Context(), kbID)
		if err != nil {
			h.logger.Error("get knowledge base failed", log.String("kb_id", kbID), log.Error(err))
			return c.JSON(http.StatusUnauthorized, domain.PWResponse{
				Success: false,
				Message: "Unauthorized",
			})
		}

		if kb.AccessSettings.IsForbidden {
			h.logger.Warn("access forbidden", log.String("kb_id", kbID))
			return c.JSON(http.StatusForbidden, domain.PWResponse{
				Success: false,
				Message: "access is forbidden",
			})
		}

		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token != "" {
				if strings.Contains(token, ".") {
					if !h.applyJWTAuth(c, kb.ID) {
						return invalidAccountSession(c)
					}
					info := domain.GetAuthInfoFromCtx(c.Request().Context())
					if requiresPasswordChange(c, info.PasswordChangeRequired) {
						return passwordChangeRequired(c)
					}
					return next(c)
				} else if h.apiTokenRepo != nil {
					apiToken, tokenErr := h.apiTokenRepo.GetByTokenWithCache(c.Request().Context(), token)
					if tokenErr == nil && apiToken != nil && apiToken.KbId == kb.ID {
						ctx := context.WithValue(c.Request().Context(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{
							IsToken:    true,
							Permission: apiToken.Permission,
							UserId:     apiToken.UserID,
							KBId:       apiToken.KbId,
						})
						c.SetRequest(c.Request().WithContext(ctx))
						return next(c)
					}
				}
				return invalidAccountSession(c)
			}
		}

		return invalidAccountSession(c)
	}
}
