package middleware

import (
	"net/http"
	"strings"

	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/pkg"
)

// Locale reads the Accept-Language header and injects the resolved locale into
// the request context via pkg.WithLocale.
//
// Resolution order:
//  1. "fr" if the best-match language tag starts with "fr"
//  2. "en" if the best-match language tag starts with "en"
//  3. "fr" as the application default
//
// Only "fr" and "en" are supported; all other language tags fall back to "fr".
//
// T5.4 observability: the resolved locale is logged at DEBUG level so
// development builds and log-aggregation systems can verify the locale
// pipeline without adding noise to production info logs.
func Locale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Accept-Language")
		locale := resolveLocale(header)
		ctx := pkg.WithLocale(r.Context(), locale)

		// T5.4: emit a structured debug event so the locale resolution is
		// traceable in development and log-aggregation systems.
		ctxutil.GetLogger(ctx).WithField("locale", locale).
			WithField("accept_language", header).
			Debug("locale: resolved")

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveLocale parses a raw Accept-Language header value and returns "fr" or "en".
func resolveLocale(header string) string {
	if header == "" {
		return "fr"
	}
	// Iterate over comma-separated language tags, pick the first recognised one.
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		lang := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
		switch lang {
		case "fr":
			return "fr"
		case "en":
			return "en"
		}
	}
	return "fr"
}
