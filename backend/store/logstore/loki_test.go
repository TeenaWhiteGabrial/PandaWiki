package logstore

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
)

func TestQueryReturnsCompleteSortedLogs(t *testing.T) {
	t.Helper()
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/loki/api/v1/query_range" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		query := r.URL.Query().Get("query")
		if !strings.Contains(query, `service="raglite"`) || !strings.Contains(query, `container=~".*rag.*"`) || !strings.Contains(query, `|= "failed"`) {
			t.Fatalf("unexpected query: %s", query)
		}
		var body bytes.Buffer
		_ = json.NewEncoder(&body).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{"result": []any{
				map[string]any{
					"stream": map[string]string{"service": "raglite", "container": "pandawiki-raglite", "stream": "stderr"},
					"values": [][]string{{"2000000000", "level=error second line\nfull details"}, {"1000000000", "first line"}},
				},
			}},
		})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(&body), Header: make(http.Header)}, nil
	})

	client := NewClient(&config.Config{Log: config.LogConfig{StoreURL: "http://loki.test"}})
	client.http.Transport = transport
	result, err := client.Query(t.Context(), domain.RuntimeLogListReq{
		Service: "raglite", Location: "rag", Keyword: "failed", PageSize: 10,
		From: time.Unix(0, 0).UTC().Format(time.RFC3339), To: time.Unix(3, 0).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Available || len(result.Items) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Items[0].EventTime.UnixNano() != 2_000_000_000 || !strings.Contains(result.Items[0].Message, "full details") {
		t.Fatalf("logs were truncated or not sorted: %#v", result.Items)
	}
	if result.Items[0].Location != "docker/raglite/pandawiki-raglite/stderr" || result.Items[0].Level != "error" {
		t.Fatalf("unexpected location or level: %#v", result.Items[0])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestQueryWithoutConfiguredStore(t *testing.T) {
	result, err := NewClient(&config.Config{}).Query(t.Context(), domain.RuntimeLogListReq{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Available || len(result.Items) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}
