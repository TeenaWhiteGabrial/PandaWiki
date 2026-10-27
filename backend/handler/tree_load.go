package handler

import (
	"log/slog"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// TrackTreeLoad starts after authentication. Response writing includes JSON
// encoding and socket writes; browser download/render times are not included.
func (h *BaseHandler) TrackTreeLoad(c echo.Context) func() {
	ctx, metrics := domain.WithTreeLoadMetrics(c.Request().Context())
	c.SetRequest(c.Request().WithContext(ctx))
	requestID := c.Response().Header().Get(echo.HeaderXRequestID)
	if requestID == "" {
		requestID = uuid.NewString()
		c.Response().Header().Set(echo.HeaderXRequestID, requestID)
	}
	return func() {
		h.baseLogger.LogAttrs(ctx, slog.LevelInfo, "document_tree_load",
			slog.String("request_id", requestID), slog.String("path", c.Path()),
			slog.Bool("success", metrics.Success),
			slog.Int("input_nodes", metrics.InputNodes), slog.Int("visible_nodes", metrics.VisibleNodes),
			slog.Any("phases_ms", metrics.PhasesMS),
			slog.Float64("total_ms", float64(time.Since(metrics.StartedAt).Microseconds())/1000),
			slog.Int64("response_bytes", c.Response().Size))
	}
}

func (h *BaseHandler) NewTreeResponseWithData(c echo.Context, data any) error {
	done := domain.MeasureTreeLoadPhase(c.Request().Context(), "response_write")
	err := h.NewResponseWithData(c, data)
	done()
	if metrics := domain.TreeLoadMetricsFromContext(c.Request().Context()); metrics != nil {
		metrics.Success = err == nil
	}
	return err
}
