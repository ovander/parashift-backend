package pkg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/ovander/backendkit/apierror"
)

// isDev returns true when APP_ENV is not "production".
func isDev() bool { return os.Getenv("APP_ENV") != "production" }

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// DecodeJSON decodes a JSON request body into v.
func DecodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// WriteError writes an error response. If err is an *apierror.AppError, its
// status code and shape are preserved. Otherwise a 500 Internal Server Error
// is returned.
//
// CR-5: In dev mode, WriteError panics if the AppError has no i18n Key set.
// This catches regressions at development time so they can never reach production.
func WriteError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apierror.AppError); ok {
		// CR-5: dev guard — every AppError must carry an i18n key so the
		// frontend can always resolve the message via t(key) rather than
		// displaying the raw English message string.
		if isDev() && appErr.Key == "" {
			panic(fmt.Sprintf(
				"[CR-5] AppError missing i18n Key — add .WithKey(\"errors.xxx\") to: %s (status %d)",
				appErr.Message,
				appErr.StatusCode,
			))
		}
		appErr.WriteJSON(w)
		return
	}
	apierror.Internal(err.Error()).WriteJSON(w)
}
