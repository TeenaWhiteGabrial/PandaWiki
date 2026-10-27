package domain

import (
	"context"
	"time"
)

type treeLoadMetricsKey struct{}

// TreeLoadMetrics contains timings and counts only, never document content or
// permission subjects. One instance belongs to one synchronous HTTP request.
type TreeLoadMetrics struct {
	StartedAt    time.Time
	PhasesMS     map[string]float64
	InputNodes   int
	VisibleNodes int
	Success      bool
}

func WithTreeLoadMetrics(ctx context.Context) (context.Context, *TreeLoadMetrics) {
	m := &TreeLoadMetrics{StartedAt: time.Now(), PhasesMS: make(map[string]float64)}
	return context.WithValue(ctx, treeLoadMetricsKey{}, m), m
}

func TreeLoadMetricsFromContext(ctx context.Context) *TreeLoadMetrics {
	m, _ := ctx.Value(treeLoadMetricsKey{}).(*TreeLoadMetrics)
	return m
}

func MeasureTreeLoadPhase(ctx context.Context, phase string) func() {
	m := TreeLoadMetricsFromContext(ctx)
	if m == nil {
		return func() {}
	}
	started := time.Now()
	return func() { m.PhasesMS[phase] += float64(time.Since(started).Microseconds()) / 1000 }
}
