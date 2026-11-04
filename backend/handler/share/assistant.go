package share

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/usecase"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

type AssistantHandler struct{}

func NewAssistantHandler(e *echo.Echo, base *handler.BaseHandler, u *usecase.AssistantUsecase, cfg *config.Config) *AssistantHandler {
	respond := func(c echo.Context, v any, err error) error {
		c.Response().Header().Set("Cache-Control", "private, no-store")
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, usecase.ErrAssistantAccess) {
				status = http.StatusForbidden
			}
			return base.NewResponseWithStatusError(c, status, usecase.AssistantErrorMessage(err), err)
		}
		return base.NewResponseWithData(c, v)
	}
	identity := func(c echo.Context, admin bool) (string, string) {
		user := domain.GetShareUserID(c)
		if admin {
			user, _ = base.V1Auth.MustGetUserID(c)
		}
		kb := c.Request().Header.Get("X-KB-ID")
		if kb == "" {
			kb = c.QueryParam("kb_id")
		}
		return kb, user
	}
	jsonLimit := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if strings.HasPrefix(c.Request().Header.Get("Content-Type"), "application/json") {
				c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 128<<10)
			}
			return next(c)
		}
	}
	share := e.Group("/share/v1/assistant", base.ShareAuthMiddleware.Authorize, echomw.BodyLimit("22M"), jsonLimit)
	share.GET("/capabilities", func(c echo.Context) error {
		return respond(c, map[string]any{"search_available": cfg.Assistant.SearchKey != "", "retention_days": cfg.Assistant.RetentionDays, "max_file_bytes": 20 << 20, "supported_formats": []string{"pdf", "docx", "pptx", "xlsx", "txt", "csv", "png", "jpg", "jpeg"}}, nil)
	})
	upload := func(admin bool) echo.HandlerFunc {
		return func(c echo.Context) error {
			kb, user := identity(c, admin)
			if kb == "" || user == "" {
				return respond(c, nil, usecase.ErrAssistantAccess)
			}
			c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, (20<<20)+(1<<20))
			if e := c.Request().ParseMultipartForm(1 << 20); e != nil {
				return respond(c, nil, fmt.Errorf("上传文件无效或超过20 MB"))
			}
			defer c.Request().MultipartForm.RemoveAll()
			file, e := c.FormFile("file")
			if e != nil {
				return respond(c, nil, e)
			}
			r, e := file.Open()
			if e != nil {
				return respond(c, nil, e)
			}
			defer r.Close()
			b, e := io.ReadAll(io.LimitReader(r, (20<<20)+1))
			if e != nil {
				return respond(c, nil, e)
			}
			v, e := u.Upload(c.Request().Context(), kb, user, file.Filename, b)
			return respond(c, v, e)
		}
	}
	share.POST("/uploads", upload(false))
	share.GET("/documents", func(c echo.Context) error {
		kb, user := identity(c, false)
		v, err := u.Documents(c.Request().Context(), kb, user, c.QueryParam("q"))
		return respond(c, v, err)
	})
	share.GET("/templates/:id/:version/content", func(c echo.Context) error {
		kb, user := identity(c, false)
		version, _ := strconv.Atoi(c.Param("version"))
		r, err := u.TemplateContent(c.Request().Context(), kb, user, c.Param("id"), version)
		if err != nil {
			return respond(c, nil, err)
		}
		defer r.Close()
		c.Response().Header().Set("Cache-Control", "private, no-store")
		c.Response().Header().Set("Content-Disposition", "attachment")
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return c.Stream(200, "application/pdf", r)
	})

	share.GET("/uploads/:id", func(c echo.Context) error {
		kb, user := identity(c, false)
		v, err := u.GetUpload(c.Request().Context(), kb, user, c.Param("id"))
		return respond(c, v, err)
	})
	share.POST("/uploads/:id/retry", func(c echo.Context) error {
		kb, user := identity(c, false)
		return respond(c, true, u.RetryUpload(c.Request().Context(), kb, user, c.Param("id")))
	})
	share.DELETE("/uploads/:id", func(c echo.Context) error {
		kb, user := identity(c, false)
		return respond(c, true, u.DeleteUpload(c.Request().Context(), kb, user, c.Param("id")))
	})
	content := func(kind string, admin bool) echo.HandlerFunc {
		return func(c echo.Context) error {
			kb, user := identity(c, admin)
			r, name, ct, err := u.Content(c.Request().Context(), kb, user, kind, c.Param("id"), c.QueryParam("preview") == "true")
			if err != nil {
				return respond(c, nil, err)
			}
			defer r.Close()
			c.Response().Header().Set("Cache-Control", "private, no-store")
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
			disp := "attachment"
			if c.QueryParam("preview") == "true" && ct == "application/pdf" {
				disp = "attachment"
			}
			c.Response().Header().Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": name}))
			return c.Stream(200, ct, r)
		}
	}
	share.GET("/uploads/:id/content", content("uploads", false))
	share.GET("/artifacts/:id/content", content("artifacts", false))
	share.GET("/artifacts/:id/sheets", func(c echo.Context) error {
		kb, user := identity(c, false)
		v, e := u.SpreadsheetPreview(c.Request().Context(), kb, user, c.Param("id"))
		return respond(c, v, e)
	})
	share.GET("/artifacts/:id", func(c echo.Context) error {
		kb, user := identity(c, false)
		v, err := u.Artifact(c.Request().Context(), kb, user, c.Param("id"))
		return respond(c, v, err)
	})
	share.GET("/templates", func(c echo.Context) error {
		kb, user := identity(c, false)
		v, err := u.Templates(c.Request().Context(), kb, user, false)
		return respond(c, v, err)
	})
	share.POST("/runs", func(c echo.Context) error {
		kb, user := identity(c, false)
		var req domain.AssistantSubmit
		if err := c.Bind(&req); err != nil {
			return respond(c, nil, err)
		}
		v, err := u.Submit(c.Request().Context(), kb, user, c.RealIP(), req)
		return respond(c, v, err)
	})
	share.GET("/runs", func(c echo.Context) error {
		kb, user := identity(c, false)
		v, err := u.Runs(c.Request().Context(), kb, user, c.QueryParam("conversation_id"))
		return respond(c, v, err)
	})
	share.GET("/runs/:id", func(c echo.Context) error {
		kb, user := identity(c, false)
		v, err := u.Run(c.Request().Context(), kb, user, c.Param("id"))
		return respond(c, v, err)
	})
	share.POST("/runs/:id/cancel", func(c echo.Context) error {
		kb, user := identity(c, false)
		return respond(c, true, u.CancelRun(c.Request().Context(), kb, user, c.Param("id")))
	})
	share.POST("/runs/:id/retry", func(c echo.Context) error {
		kb, user := identity(c, false)
		return respond(c, true, u.RetryRun(c.Request().Context(), kb, user, c.Param("id")))
	})
	share.POST("/runs/:id/continue", func(c echo.Context) error {
		kb, user := identity(c, false)
		var input domain.AssistantContinue
		if err := c.Bind(&input); err != nil {
			return respond(c, nil, err)
		}
		v, err := u.ContinueRun(c.Request().Context(), kb, user, c.Param("id"), input)
		return respond(c, v, err)
	})
	share.GET("/preferences/:id", func(c echo.Context) error {
		kb, user := identity(c, false)
		v, err := u.Preferences(c.Request().Context(), kb, user, c.Param("id"), nil)
		return respond(c, v, err)
	})
	share.PATCH("/preferences/:id", func(c echo.Context) error {
		kb, user := identity(c, false)
		var settings domain.JSONB
		if err := c.Bind(&settings); err != nil {
			return respond(c, nil, err)
		}
		v, err := u.Preferences(c.Request().Context(), kb, user, c.Param("id"), &settings)
		return respond(c, v, err)
	})
	admin := e.Group("/api/v1/assistant", base.V1Auth.Authorize, base.V1Auth.ValidateUserRole(consts.UserRoleAdmin), echomw.BodyLimit("22M"), jsonLimit)
	admin.POST("/uploads", upload(true))
	admin.GET("/uploads/:id", func(c echo.Context) error {
		kb, user := identity(c, true)
		v, err := u.GetUpload(c.Request().Context(), kb, user, c.Param("id"))
		return respond(c, v, err)
	})
	admin.GET("/uploads/:id/content", content("uploads", true))
	admin.GET("/templates", func(c echo.Context) error {
		kb, user := identity(c, true)
		v, err := u.Templates(c.Request().Context(), kb, user, true)
		return respond(c, v, err)
	})
	admin.POST("/templates", func(c echo.Context) error {
		kb, user := identity(c, true)
		var t domain.AssistantTemplate
		if err := c.Bind(&t); err != nil {
			return respond(c, nil, err)
		}
		v, err := u.SaveTemplate(c.Request().Context(), kb, user, t)
		return respond(c, v, err)
	})
	admin.PUT("/templates/:id/permissions", func(c echo.Context) error {
		kb, _ := identity(c, true)
		var permissions domain.NodePermissions
		if e := c.Bind(&permissions); e != nil {
			return respond(c, nil, e)
		}
		return respond(c, true, u.TemplatePermissions(c.Request().Context(), kb, c.Param("id"), permissions))
	})
	admin.GET("/templates/:id/:version/content", func(c echo.Context) error {
		kb, user := identity(c, true)
		version, _ := strconv.Atoi(c.Param("version"))
		r, err := u.TemplateContent(c.Request().Context(), kb, user, c.Param("id"), version)
		if err != nil {
			return respond(c, nil, err)
		}
		defer r.Close()
		c.Response().Header().Set("Content-Disposition", "attachment")
		c.Response().Header().Set("Cache-Control", "private, no-store")
		return c.Stream(200, "application/pdf", r)
	})
	admin.PATCH("/templates/:id/:version", func(c echo.Context) error {
		kb, _ := identity(c, true)
		var req struct {
			State     string `json:"state"`
			IsDefault bool   `json:"is_default"`
		}
		if err := c.Bind(&req); err != nil {
			return respond(c, nil, err)
		}
		version, _ := strconv.Atoi(c.Param("version"))
		return respond(c, true, u.TemplateState(c.Request().Context(), kb, c.Param("id"), version, req.State, req.IsDefault))
	})
	return &AssistantHandler{}
}
