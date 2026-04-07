package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ovander/parashift/internal/pkg"
)

// DebugHandler exposes dev-only diagnostic endpoints.
// It is only registered when APP_ENV != "production".
type DebugHandler struct{}

// NewDebugHandler creates a DebugHandler.
func NewDebugHandler() *DebugHandler { return &DebugHandler{} }

// DecodeToken decodes a JWT without validation and returns the header + claims.
// This lets you instantly inspect what Socrate put in the token (alg, kid, iss, role…)
// without needing jwt.io or any external tooling.
//
//	GET /debug/token
//	Authorization: Bearer <access_token>
func (h *DebugHandler) DecodeToken(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		pkg.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing Authorization header"})
		return
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		pkg.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "not a JWT — expected 3 dot-separated parts",
			"parts": strings.Join([]string{"got", string(rune('0'+len(parts))), "parts"}, " "),
		})
		return
	}

	header, errH := decodeSegment(parts[0])
	claims, errC := decodeSegment(parts[1])

	out := map[string]interface{}{
		"header": header,
		"claims": claims,
		"note":   "signature NOT verified — diagnostic only",
	}
	if errH != nil {
		out["header_decode_error"] = errH.Error()
	}
	if errC != nil {
		out["claims_decode_error"] = errC.Error()
	}
	pkg.WriteJSON(w, http.StatusOK, out)
}

func decodeSegment(seg string) (map[string]interface{}, error) {
	// Base64url decode, padding-tolerant
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
