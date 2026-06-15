package share

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/usecase"
)

type ShareContributeHandler struct {
	*handler.BaseHandler
	logger  *log.Logger
	usecase *usecase.ContributeUsecase
	app     *usecase.AppUsecase
}

func NewShareContributeHandler(
	e *echo.Echo,
	baseHandler *handler.BaseHandler,
	logger *log.Logger,
	usecase *usecase.ContributeUsecase,
	app *usecase.AppUsecase,
) *ShareContributeHandler {
	h := &ShareContributeHandler{
		BaseHandler: baseHandler,
		logger:      logger.WithModule("handler.share.contribute"),
		usecase:     usecase,
		app:         app,
	}

	group := e.Group("/share/pro/v1/contribute", h.ShareAuthMiddleware.Authorize, h.ShareAuthMiddleware.CheckForbidden,
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Response().Header().Set("Access-Control-Allow-Origin", "*")
				c.Response().Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
				c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Origin, Accept, Authorization, X-KB-ID")
				if c.Request().Method == "OPTIONS" {
					return c.NoContent(http.StatusOK)
				}
				return next(c)
			}
		},
	)
	group.POST("/submit", h.Submit)

	return h
}

func (h *ShareContributeHandler) Submit(c echo.Context) error {
	kbID := c.Request().Header.Get("X-KB-ID")
	if kbID == "" {
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}

	var req domain.SubmitContributeReq
	if err := c.Bind(&req); err != nil {
		return h.NewResponseWithError(c, "bind request failed", err)
	}
	if err := c.Validate(&req); err != nil {
		return h.NewResponseWithError(c, "invalid request", err)
	}
	if !h.Captcha.ValidateToken(c.Request().Context(), req.CaptchaToken) {
		return h.NewResponseWithError(c, "failed to validate captcha token", nil)
	}

	authInfo := domain.GetAuthInfoFromCtx(c.Request().Context())
	userID := ""
	userName := ""
	if authInfo != nil {
		userID = authInfo.UserId
	}
	if req.Name != "" {
		userName = req.Name
	}

	id, err := h.usecase.Submit(c.Request().Context(), &req, kbID, userID, userName, c.RealIP())
	if err != nil {
		return h.NewResponseWithError(c, "submit contribute failed", err)
	}
	return h.NewResponseWithData(c, &domain.SubmitContributeResp{ID: id})
}
