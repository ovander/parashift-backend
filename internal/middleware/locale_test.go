package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ovander/parashift/internal/pkg"
	"github.com/stretchr/testify/assert"
)

// captureLocale is a test handler that captures the locale injected by the
// Locale middleware and writes it to the response body.
func captureLocale(w http.ResponseWriter, r *http.Request) {
	locale := pkg.GetLocale(r.Context())
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(locale)) //nolint:errcheck
}

func TestLocale_Middleware(t *testing.T) {
	handler := Locale(http.HandlerFunc(captureLocale))

	tests := []struct {
		name           string
		acceptLanguage string
		wantLocale     string
	}{
		{
			name:           "French header → fr",
			acceptLanguage: "fr-FR,fr;q=0.9,en;q=0.8",
			wantLocale:     "fr",
		},
		{
			name:           "English header → en",
			acceptLanguage: "en-GB,en;q=0.9",
			wantLocale:     "en",
		},
		{
			name:           "English-US header → en",
			acceptLanguage: "en-US",
			wantLocale:     "en",
		},
		{
			name:           "Unsupported language defaults to fr",
			acceptLanguage: "de-DE,de;q=0.9",
			wantLocale:     "fr",
		},
		{
			name:           "Empty header defaults to fr",
			acceptLanguage: "",
			wantLocale:     "fr",
		},
		{
			name:           "French before English picks fr",
			acceptLanguage: "fr;q=1.0,en;q=0.8",
			wantLocale:     "fr",
		},
		{
			name:           "Wildcard header defaults to fr",
			acceptLanguage: "*",
			wantLocale:     "fr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, tt.wantLocale, rr.Body.String())
		})
	}
}

func TestResolveLocale(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"fr",          "fr"},
		{"en",          "en"},
		{"fr-BE",       "fr"},
		{"en-US",       "en"},
		{"",            "fr"},
		{"de",          "fr"},
		{"zh-CN",       "fr"},
		{"fr,en;q=0.5", "fr"},
		{"en,fr;q=0.5", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveLocale(tt.header))
		})
	}
}
