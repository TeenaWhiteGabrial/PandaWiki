package usecase

import (
	"context"
	"github.com/chaitin/panda-wiki/domain"
)

type fileExtractionReporterKey struct{}

func WithFileExtractionReporter(ctx context.Context, report func(*domain.FileExtraction)) context.Context {
	return context.WithValue(ctx, fileExtractionReporterKey{}, report)
}

func reportFileExtraction(ctx context.Context, result *domain.FileExtraction) {
	if report, ok := ctx.Value(fileExtractionReporterKey{}).(func(*domain.FileExtraction)); ok {
		report(result)
	}
}

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
