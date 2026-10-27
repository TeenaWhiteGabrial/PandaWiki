package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/labstack/echo/v4"
)

func TestTreeLoadResponseMetrics(t *testing.T) {
	var output bytes.Buffer
	h := &BaseHandler{baseLogger: &log.Logger{Logger: slog.New(slog.NewJSONHandler(&output, nil))}}
	c := echo.New().NewContext(httptest.NewRequest("GET", "/share/v1/node/list", nil), httptest.NewRecorder())
	c.SetPath("/share/v1/node/list")
	finish := h.TrackTreeLoad(c)
	m := domain.TreeLoadMetricsFromContext(c.Request().Context())
	m.InputNodes, m.VisibleNodes = 42, 7
	domain.MeasureTreeLoadPhase(c.Request().Context(), "nodes_query")()
	if err := h.NewTreeResponseWithData(c, []string{"private-document-title"}); err != nil {
		t.Fatal(err)
	}
	finish()
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["msg"] != "document_tree_load" || record["success"] != true || record["input_nodes"] != float64(42) || record["visible_nodes"] != float64(7) {
		t.Fatalf("missing timing metadata: %+v", record)
	}
	if record["request_id"] != c.Response().Header().Get(echo.HeaderXRequestID) || record["request_id"] == "" {
		t.Fatal("response and log must carry the same request ID")
	}
	phases := record["phases_ms"].(map[string]any)
	if phases["response_write"] == nil || phases["nodes_query"] == nil || record["response_bytes"].(float64) <= 0 {
		t.Fatalf("missing response/query timings: %+v", record)
	}
	if strings.Contains(output.String(), "private-document-title") {
		t.Fatal("performance logs must not contain document content")
	}
}
