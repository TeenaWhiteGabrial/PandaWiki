package usecase

import "context"

type learningStageReporterKey struct{}

// WithLearningStageReporter adds optional instrumentation to file extraction.
func WithLearningStageReporter(ctx context.Context, report func(string, string)) context.Context {
	return context.WithValue(ctx, learningStageReporterKey{}, report)
}

func reportLearningStage(ctx context.Context, stage, message string) {
	if report, ok := ctx.Value(learningStageReporterKey{}).(func(string, string)); ok {
		report(stage, message)
	}
}
