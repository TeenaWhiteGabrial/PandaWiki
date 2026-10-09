package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chaitin/panda-wiki/domain"
)

type pdfParserStatus struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Error  string `json:"error"`
	Events []struct {
		Sequence int    `json:"sequence"`
		Stage    string `json:"stage"`
		Message  string `json:"message"`
	} `json:"events"`
	Result *domain.FileExtraction `json:"result"`
}

func parsePDFWithService(ctx context.Context, filePath, filename string) directParseResult {
	baseURL := strings.TrimRight(os.Getenv("PDF_PARSER_URL"), "/")
	if baseURL == "" {
		return directParseResult{Message: "PDF 解析服务未配置（PDF_PARSER_URL），请启动 pdf-parser 后重试"}
	}
	client := &http.Client{Timeout: 90 * time.Second}
	request := func(ctx context.Context, method, path string, body io.Reader, size int64) (*http.Response, error) {
		if body != nil {
			body = io.NopCloser(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.ContentLength = size
			req.Header.Set("Content-Type", "application/pdf")
		}
		if token := os.Getenv("PDF_PARSER_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return client.Do(req)
	}
	fail := func(err error) directParseResult {
		return directParseResult{Message: "PDF 正文解析失败：" + err.Error()}
	}
	f, err := os.Open(filePath)
	if err != nil {
		return fail(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fail(err)
	}
	var job pdfParserStatus
	for {
		reportLearningStage(ctx, "pdf_waiting", "提交至独立 PDF 解析服务，等待解析名额")
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return fail(err)
		}
		resp, err := request(ctx, http.MethodPost, "/jobs", f, info.Size())
		if err != nil {
			return fail(fmt.Errorf("无法连接解析服务：%w", err))
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		resp.Body.Close()
		if readErr != nil {
			return fail(readErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			select {
			case <-ctx.Done():
				return fail(ctx.Err())
			case <-time.After(2 * time.Second):
				continue
			}
		}
		if resp.StatusCode != http.StatusAccepted {
			return fail(fmt.Errorf("解析服务返回 HTTP %d：%s", resp.StatusCode, data))
		}
		if err := json.Unmarshal(data, &job); err != nil || job.ID == "" {
			return fail(fmt.Errorf("解析服务未返回有效任务 ID"))
		}
		break
	}
	jobPath := "/jobs/" + job.ID
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if resp, err := request(cleanupCtx, http.MethodDelete, jobPath, nil, 0); err == nil {
			resp.Body.Close()
		}
	}()
	sequence := 0
	for {
		pollCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		resp, err := request(pollCtx, http.MethodGet, jobPath, nil, 0)
		if err != nil {
			cancel()
			return fail(err)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
		resp.Body.Close()
		cancel()
		if err != nil {
			return fail(err)
		}
		if resp.StatusCode != http.StatusOK {
			return fail(fmt.Errorf("解析任务查询返回 HTTP %d：%s", resp.StatusCode, data))
		}
		if err := json.Unmarshal(data, &job); err != nil {
			return fail(err)
		}
		for _, event := range job.Events {
			if event.Sequence > sequence {
				reportLearningStage(ctx, event.Stage, event.Message)
				sequence = event.Sequence
			}
		}
		if job.State == "failed" {
			return fail(fmt.Errorf("%s", job.Error))
		}
		if job.State == "completed" {
			if job.Result == nil || (job.Result.Scope != "full" && job.Result.Scope != "partial") {
				return fail(fmt.Errorf("解析服务返回无效的正文覆盖信息"))
			}
			resp, err := request(ctx, http.MethodGet, jobPath+"/text", nil, 0)
			if err != nil {
				return fail(err)
			}
			text, err := io.ReadAll(io.LimitReader(resp.Body, directParseMaxTextBytes+1))
			resp.Body.Close()
			if err != nil {
				return fail(err)
			}
			if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(text)) == "" {
				return fail(fmt.Errorf("解析服务没有返回有效正文"))
			}
			content, truncated := limitParsedMarkdown(withDocumentTitle(filename, string(text)))
			if truncated {
				job.Result.Truncated = true
				job.Result.Scope = "partial"
			}
			return directParseResult{Content: content, OK: true, Message: job.Result.Message, Truncated: job.Result.Truncated, Extraction: job.Result}
		}
		select {
		case <-ctx.Done():
			return fail(ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
