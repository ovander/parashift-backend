// export_test.go — white-box exports for the service_test package.
// This file is compiled only during testing; it has zero impact on production builds.
package service

import (
	"time"

	"github.com/ovander/parashift/internal/model"
	"github.com/sirupsen/logrus"
)

// LanguageInstruction exposes the unexported languageInstruction helper for tests.
func LanguageInstruction(locale string) string {
	return languageInstruction(locale)
}

// DetectLanguage exposes the unexported detectLanguage helper for tests.
func DetectLanguage(text string) string {
	return detectLanguage(text)
}

// EnsureLanguage exposes the unexported ensureLanguage helper for tests.
func EnsureLanguage(text, locale string, logger *logrus.Entry) string {
	return ensureLanguage(text, locale, logger)
}

// BuildSuggestPromptForTest exposes buildSuggestPrompt with nil-safe defaults.
func BuildSuggestPromptForTest(locale string, shift *model.ShiftInstance, employees []*model.Employee) string {
	if shift == nil {
		shift = &model.ShiftInstance{
			Date:                  time.Now(),
			StartTime:             "08:00",
			EndTime:               "16:00",
			RequiredQualification: "",
		}
	}
	if employees == nil {
		employees = []*model.Employee{}
	}
	return buildSuggestPrompt(locale, shift, employees)
}

// BuildOptimizePromptForTest exposes buildOptimizePrompt with nil-safe defaults.
func BuildOptimizePromptForTest(locale string, shifts []*model.ShiftInstance, employees []*model.Employee) string {
	if shifts == nil {
		shifts = []*model.ShiftInstance{}
	}
	if employees == nil {
		employees = []*model.Employee{}
	}
	from := time.Now()
	to := from.AddDate(0, 0, 7)
	return buildOptimizePrompt(locale, from, to, shifts, employees)
}

// BuildInsightPromptForTest exposes buildInsightPrompt with nil-safe defaults.
func BuildInsightPromptForTest(locale string, shifts []*model.ShiftInstance, employees []*model.Employee) string {
	if shifts == nil {
		shifts = []*model.ShiftInstance{}
	}
	if employees == nil {
		employees = []*model.Employee{}
	}
	from := time.Now()
	to := from.AddDate(0, 0, 7)
	return buildInsightPrompt(locale, from, to, shifts, employees)
}
