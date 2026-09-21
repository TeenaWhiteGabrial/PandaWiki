package logstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
)

const maxQueryLimit = 2000

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(cfg.Log.StoreURL), "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Available() bool { return c != nil && c.baseURL != "" }

type lokiResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

func (c *Client) Query(ctx context.Context, req domain.RuntimeLogListReq) (*domain.RawRuntimeLogListResp, error) {
	result := &domain.RawRuntimeLogListResp{Items: make([]domain.RawRuntimeLog, 0), Available: c.Available()}
	if !c.Available() {
		return result, nil
	}

	limit := req.PageSize
	if limit <= 0 {
		limit = 500
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	matchers := []string{`application="pandawiki"`}
	if service := strings.TrimSpace(req.Service); service != "" {
		matchers = append(matchers, "service="+strconv.Quote(service))
	}
	if location := strings.TrimSpace(req.Location); location != "" {
		matchers = append(matchers, "container=~"+strconv.Quote(".*"+regexp.QuoteMeta(location)+".*"))
	}
	selector := "{" + strings.Join(matchers, ",") + "}"
	query := selector
	if keyword := strings.TrimSpace(req.Keyword); keyword != "" {
		query += " |= " + strconv.Quote(keyword)
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("direction", "BACKWARD")
	params.Set("limit", strconv.Itoa(limit+1))
	if from, ok := parseQueryTime(req.From); ok {
		params.Set("start", strconv.FormatInt(from.UnixNano(), 10))
	}
	if to, ok := parseQueryTime(req.To); ok {
		params.Set("end", strconv.FormatInt(to.UnixNano(), 10))
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/loki/api/v1/query_range?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query log store: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query log store: status %d", resp.StatusCode)
	}
	var payload lokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode log store response: %w", err)
	}
	if payload.Status != "success" {
		return nil, fmt.Errorf("query log store: status %s", payload.Status)
	}

	for _, stream := range payload.Data.Result {
		for _, value := range stream.Values {
			if len(value) != 2 {
				continue
			}
			nanoseconds, err := strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				continue
			}
			service := stream.Stream["service"]
			container := strings.TrimPrefix(stream.Stream["container"], "/")
			logStream := stream.Stream["stream"]
			locationType := stream.Stream["location"]
			if locationType == "" {
				locationType = "docker"
			}
			result.Items = append(result.Items, domain.RawRuntimeLog{
				EventTime: time.Unix(0, nanoseconds),
				Level:     detectLevel(value[1]),
				Service:   service,
				Container: container,
				Stream:    logStream,
				Location:  strings.Join(nonEmpty(locationType, service, container, logStream), "/"),
				Message:   value[1],
				Labels:    stream.Stream,
			})
		}
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].EventTime.After(result.Items[j].EventTime) })
	if len(result.Items) > limit {
		result.Items = result.Items[:limit]
		result.HasMore = true
	}
	return result, nil
}

func parseQueryTime(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, value)
	return parsed, err == nil
}

func detectLevel(message string) string {
	value := strings.ToLower(message)
	for _, level := range []string{"fatal", "panic", "error", "warn", "debug", "trace"} {
		if strings.Contains(value, "level="+level) || strings.Contains(value, `\"level\":\"`+level+`\"`) || strings.Contains(value, "["+level+"]") {
			return level
		}
	}
	return "info"
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
