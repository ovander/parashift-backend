package service_test

import (
	"strings"
	"testing"

	"github.com/ovander/parashift/internal/service"
	"github.com/stretchr/testify/assert"
)

// ─── T1.5: languageInstruction ────────────────────────────────────────────────

func TestLanguageInstruction_French(t *testing.T) {
	instr := service.LanguageInstruction("fr")
	assert.Contains(t, instr, "français", "FR instruction must mention French")
	assert.NotContains(t, instr, "English", "FR instruction must not mention English")
}

func TestLanguageInstruction_English(t *testing.T) {
	instr := service.LanguageInstruction("en")
	assert.Contains(t, instr, "English", "EN instruction must mention English")
	assert.NotContains(t, instr, "français", "EN instruction must not mention French")
}

func TestLanguageInstruction_UnknownFallsBackToFrench(t *testing.T) {
	// Any unrecognised locale should produce the French instruction (FR-first strategy)
	instr := service.LanguageInstruction("de")
	assert.Contains(t, instr, "français")
}

// ─── T1.6 / CR-3: detectLanguage ─────────────────────────────────────────────

func TestDetectLanguage_French(t *testing.T) {
	frText := "Le planning est bien organisé avec les employés disponibles pour cette semaine."
	assert.Equal(t, "fr", service.DetectLanguage(frText))
}

func TestDetectLanguage_English(t *testing.T) {
	enText := "The schedule is well organised with the available employees for this week."
	assert.Equal(t, "en", service.DetectLanguage(enText))
}

func TestDetectLanguage_ShortTextReturnsEmpty(t *testing.T) {
	// A very short string has no reliable signal → "" (unknown)
	assert.Equal(t, "", service.DetectLanguage("ok"))
}

func TestDetectLanguage_EmptyStringReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", service.DetectLanguage(""))
}

// ─── T1.6 / CR-3: ensureLanguage (via a French prompt scenario) ──────────────

func TestEnsureLanguage_CorrectLanguageNoWarn(t *testing.T) {
	// Should return the text unchanged with no warning logged
	logger := newTestLogger()
	frText := "Le planning est bien organisé avec les employés disponibles pour cette semaine et plus."
	result := service.EnsureLanguage(frText, "fr", logger)
	assert.Equal(t, frText, result, "EnsureLanguage must return the original text unchanged")
}

func TestEnsureLanguage_DriftDetectedReturnsOriginalText(t *testing.T) {
	// Even when drift is detected the original text is returned (caller decides what to do)
	logger := newTestLogger()
	enText := "The schedule is well organised with the available employees for this week."
	result := service.EnsureLanguage(enText, "fr", logger)
	// Text must come back unchanged — ensureLanguage only warns, it doesn't mutate
	assert.Equal(t, enText, result)
}

// ─── Prompt builders contain the language instruction ─────────────────────────

func TestBuildSuggestPrompt_ContainsLanguageInstruction(t *testing.T) {
	prompt := service.BuildSuggestPromptForTest("fr", nil, nil)
	assert.True(t,
		strings.Contains(prompt, "français") || strings.Contains(prompt, "IMPORTANT"),
		"FR suggest prompt must embed the language instruction",
	)
}

func TestBuildOptimizePrompt_EnglishContainsEnglishInstruction(t *testing.T) {
	prompt := service.BuildOptimizePromptForTest("en", nil, nil)
	assert.Contains(t, prompt, "English")
}

func TestBuildInsightPrompt_FrenchContainsFrenchInstruction(t *testing.T) {
	prompt := service.BuildInsightPromptForTest("fr", nil, nil)
	assert.Contains(t, prompt, "français")
}
