package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

type RuntimeEntry struct {
	Time    time.Time
	Level   string
	Service string
	Module  string
	Source  string
	Message string
	Fields  map[string]any
}

type runtimeSink struct {
	write func(context.Context, RuntimeEntry) error
}

var (
	boundRuntimeSink atomic.Pointer[runtimeSink]
	runtimeQueue     = make(chan RuntimeEntry, 2048)
	eventCodePattern = regexp.MustCompile(`[^a-z0-9]+`)
)

func init() {
	go func() {
		for entry := range runtimeQueue {
			sink := boundRuntimeSink.Load()
			if sink == nil || sink.write == nil {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = sink.write(ctx, entry)
			cancel()
		}
	}()
}

func BindRuntimeSink(write func(context.Context, RuntimeEntry) error) {
	boundRuntimeSink.Store(&runtimeSink{write: write})
}

type runtimeHandler struct {
	next   slog.Handler
	attrs  []slog.Attr
	groups []string
}

func newRuntimeHandler(next slog.Handler) slog.Handler { return &runtimeHandler{next: next} }

func (h *runtimeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Persist every emitted level. The wrapped handler still decides what is
	// printed to stdout according to LOG_LEVEL.
	return true
}

func (h *runtimeHandler) Handle(ctx context.Context, record slog.Record) error {
	var err error
	if h.next.Enabled(ctx, record.Level) {
		err = h.next.Handle(ctx, record)
	}
	entry := RuntimeEntry{
		Time:    record.Time,
		Level:   strings.ToLower(record.Level.String()),
		Service: strings.TrimSpace(os.Getenv("SERVICE_NAME")),
		Message: record.Message,
		Fields:  map[string]any{},
	}
	if record.PC != 0 {
		frame, _ := runtime.CallersFrames([]uintptr{record.PC}).Next()
		entry.Source = fmt.Sprintf("%s:%d", frame.File, frame.Line)
	}
	if entry.Service == "" {
		entry.Service = "backend"
	}
	for _, attr := range h.attrs {
		appendSafeAttr(&entry, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendSafeAttr(&entry, attr)
		return true
	})
	select {
	case runtimeQueue <- entry:
	default:
	}
	return err
}

func (h *runtimeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.next = h.next.WithAttrs(attrs)
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *runtimeHandler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.next = h.next.WithGroup(name)
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

func appendSafeAttr(entry *RuntimeEntry, attr slog.Attr) {
	key := strings.ToLower(attr.Key)
	if key == "module" {
		entry.Module = attr.Value.String()
		return
	}
	for _, blocked := range []string{"password", "secret", "token", "authorization", "cookie", "content", "schema", "document", "prompt", "request", "response", "summary"} {
		if strings.Contains(key, blocked) {
			return
		}
	}
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		for _, child := range value.Group() {
			appendSafeAttr(entry, child)
		}
		return
	}
	entry.Fields[attr.Key] = fmt.Sprint(value.Any())
}

func EventCode(module, message string) string {
	value := strings.ToLower(strings.TrimSpace(module + "_" + message))
	value = strings.Trim(eventCodePattern.ReplaceAllString(value, "_"), "_")
	if len(value) > 128 {
		value = value[:128]
	}
	return value
}
