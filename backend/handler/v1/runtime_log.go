package v1

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/middleware"
	"github.com/chaitin/panda-wiki/repo/pg"
	"github.com/chaitin/panda-wiki/store/logstore"
)

type RuntimeLogHandler struct{}

func NewRuntimeLogHandler(e *echo.Echo, base *handler.BaseHandler, auth middleware.AuthMiddleware, repo *pg.SystemSettingRepo, cfg *config.Config) *RuntimeLogHandler {
	logClient := logstore.NewClient(cfg)
	group := e.Group("/api/v1/system/runtime-logs", auth.Authorize, auth.ValidateUserRole(consts.UserRoleAdmin))
	group.GET("", func(c echo.Context) error {
		var req domain.RuntimeLogListReq
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, domain.PWResponse{Success: false, Message: "日志查询参数不正确"})
		}
		if req.Scope == "system" {
			result, err := logClient.Query(c.Request().Context(), req)
			if err != nil {
				return c.JSON(http.StatusServiceUnavailable, domain.PWResponse{Success: false, Message: "集中日志服务暂不可用"})
			}
			return base.NewResponseWithData(c, result)
		}
		result, err := repo.ListRuntimeLogs(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, domain.PWResponse{Success: false, Message: "读取运行日志失败"})
		}
		return base.NewResponseWithData(c, result)
	})
	return &RuntimeLogHandler{}
}
