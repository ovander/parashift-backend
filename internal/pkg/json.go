package pkg

import (
	"encoding/json"
	"net/http"

	"github.com/ovander/backendkit/apierror"
)

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
func WriteError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*apierror.AppError); ok {
		appErr.WriteJSON(w)
		return
	}
	apierror.Internal(err.Error()).WriteJSON(w)
}
