package share

import (
	"errors"
	"io"

	"github.com/labstack/echo/v4"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/usecase"
)

type OpenapiV1Handler struct {
	*handler.BaseHandler
	logger  *log.Logger
	appCase *usecase.AppUsecase
}

func NewOpenapiV1Handler(
	e *echo.Echo,
	baseHandler *handler.BaseHandler,
	logger *log.Logger,
	appCase *usecase.AppUsecase,
) *OpenapiV1Handler {
	h := &OpenapiV1Handler{
		BaseHandler: baseHandler,
		logger:      logger,
		appCase:     appCase,
	}

	OpenapiGroup := e.Group("/share/v1/openapi")

	// lark机器人
	OpenapiGroup.POST("/lark/bot/:kb_id", h.LarkBot)

	return h
}

// LarkBot Lark机器人请求
//
//	@Tags			ShareOpenapi
//	@Summary		Lark机器人请求
//	@Description	Lark机器人请求
//	@ID				v1-LarkBot
//	@Accept			json
//	@Produce		json
//	@Param			kb_id	path		string	true	"知识库ID"
//	@Success		200		{object}	domain.PWResponse
//	@Router			/share/v1/openapi/lark/bot/{kb_id} [post]
func (h *OpenapiV1Handler) LarkBot(c echo.Context) error {
	ctx := c.Request().Context()

	kbID := c.Param("kb_id")
	if kbID == "" {
		h.logger.Error("kb_id is required")
		return h.NewResponseWithError(c, "kb_id is required", nil)
	}

	// 获取应用配置
	appInfo, err := h.appCase.GetAppDetailByKBIDAndAppType(ctx, kbID, domain.AppTypeLarkBot)
	if err != nil {
		h.logger.Error("failed to get app detail", log.Error(err), log.String("kb_id", kbID))
		return h.NewResponseWithError(c, "failed to get app detail", err)
	}

	if appInfo.Settings.LarkBotSettings.IsEnabled == nil || !*appInfo.Settings.LarkBotSettings.IsEnabled {
		h.logger.Error("lark bot is not enabled")
		return h.NewResponseWithError(c, "lark bot is not enabled", err)
	}

	var eventHandler *dispatcher.EventDispatcher
	client, ok := h.appCase.GetLarkBotClient(appInfo.ID)
	if ok {
		eventHandler = client.GetEventHandler()
	}

	if eventHandler == nil {
		eventHandler = dispatcher.NewEventDispatcher(
			appInfo.Settings.LarkBotSettings.VerifyToken,
			appInfo.Settings.LarkBotSettings.EncryptKey,
		)
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		h.logger.Error("failed to read request body", log.Error(err))
		return h.NewResponseWithError(c, "failed to read request body", err)
	}
	defer c.Request().Body.Close()

	eventReq := &larkevent.EventReq{
		Header:     c.Request().Header,
		Body:       body,
		RequestURI: c.Request().RequestURI,
	}

	eventResp := eventHandler.Handle(ctx, eventReq)
	if eventResp == nil {
		h.logger.Error("failed to handle lark event: nil response")
		return h.NewResponseWithError(c, "failed to handle lark event", errors.New("nil response"))
	}

	for key, values := range eventResp.Header {
		for _, value := range values {
			c.Response().Header().Add(key, value)
		}
	}

	return c.JSONBlob(eventResp.StatusCode, eventResp.Body)
}
