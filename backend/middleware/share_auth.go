package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo-contrib/session"
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
	})
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

	ctx := context.WithValue(c.Request().Context(), domain.CtxAuthInfoKey, &domain.CtxAuthInfo{
		IsToken:    false,
		Permission: consts.UserKBPermissionDocManage,
		UserId:     userID,
		KBId:       kbID,
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

func (h *ShareAuthMiddleware) Authorize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
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

		authHeader := c.Request().Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token != "" {
				if strings.Contains(token, ".") {
					if h.applyJWTAuth(c, kb.ID) {
						return next(c)
					}
					ctxAuthInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
					if ctxAuthInfo != nil && !ctxAuthInfo.IsToken {
						if ctxAuthInfo.KBId == "" {
							ctxAuthInfo.KBId = kb.ID
						}
						if ctxAuthInfo.Permission == consts.UserKBPermissionNull {
							ctxAuthInfo.Permission = consts.UserKBPermissionDocManage
						}
						return next(c)
					}
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
			}
		}

		// 分享站点允许匿名访问，由后续栏目/文档权限规则决定可见范围。
		// 已登录用户仍会透传身份，用于访问有权限要求的内容。
		if !kb.AccessSettings.EnterpriseAuth.Enabled && !kb.AccessSettings.SimpleAuth.Enabled {
			return next(c)
		}

		sess, err := session.Get(domain.SessionName, c)
		if err != nil {
			h.logger.Warn("session get failed for anonymous share access", log.Error(err))
			return next(c)
		}

		KbIDSess, ok := sess.Values["kb_id"].(string)
		if !ok || kbID == "" || KbIDSess != kb.ID {
			return next(c)
		}

		if kb.AccessSettings.EnterpriseAuth.Enabled {
			return next(c)
		}

		return next(c)
	}
}
