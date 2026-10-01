package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ovander/backendkit/socrate"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/sirupsen/logrus"
)

// SocrateTokens is the part of *socrate.Client the /auth routes use: the
// authorization-code exchange, refresh and revocation, each sent to Socrate's
// public OAuth endpoints with the client secret.
type SocrateTokens interface {
	ExchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*socrate.TokenSet, error)
	RefreshToken(ctx context.Context, refreshToken string) (*socrate.TokenSet, error)
	RevokeToken(ctx context.Context, token string) error
}

// AuthHandler handles OAuth2 token exchange with Socrate.
// It follows the same pattern as Ascenda/KerPlan:
//   - /auth/callback  → exchange code for tokens (lean, no DB)
//   - /auth/refresh   → rotate tokens
//   - /auth/logout    → revoke refresh token
//
// User enrichment (ParaShift role, store_id) is the responsibility of GET /me,
// which is called by the frontend immediately after the callback.
type AuthHandler struct {
	tokens      SocrateTokens // nil when Socrate is not configured
	redirectURL string        // SOCRATE_REDIRECT_URL, used when the request names none
}

// NewAuthHandler creates a new AuthHandler. tokens nil (Socrate not
// configured) makes every route answer 503.
func NewAuthHandler(tokens SocrateTokens, redirectURL string) *AuthHandler {
	return &AuthHandler{tokens: tokens, redirectURL: redirectURL}
}

// ── Request / response types ──────────────────────────────────────────────────

type callbackRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"codeVerifier"`
	RedirectURI  string `json:"redirectUri"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// authTokens is what the frontend expects from /auth/callback and /auth/refresh.
type authTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func (h *AuthHandler) unavailable(w http.ResponseWriter) bool {
	if h.tokens != nil {
		return false
	}
	pkg.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "authentication is not configured"})
	return true
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Callback exchanges a PKCE authorization code for a Socrate token pair.
// POST /auth/callback  { code, codeVerifier, redirectUri }
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	var req callbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		pkg.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Code == "" || req.CodeVerifier == "" {
		pkg.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "code and codeVerifier are required"})
		return
	}

	redirectURI := req.RedirectURI
	if redirectURI == "" {
		redirectURI = h.redirectURL
	}

	tokens, err := h.tokens.ExchangeCode(r.Context(), req.Code, redirectURI, req.CodeVerifier)
	if err != nil {
		// Log upstream detail server-side; return a generic message (SEC-8).
		logrus.WithError(err).Warn("auth: token exchange failed")
		pkg.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "authentication failed"})
		return
	}

	pkg.WriteJSON(w, http.StatusOK, authTokens{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Refresh rotates a Socrate refresh token into a new token pair.
// POST /auth/refresh  { refreshToken }
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.unavailable(w) {
		return
	}
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		pkg.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "refreshToken is required"})
		return
	}

	tokens, err := h.tokens.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		// Log upstream detail server-side; return a generic message (SEC-8).
		logrus.WithError(err).Warn("auth: token refresh failed")
		pkg.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "token refresh failed"})
		return
	}

	// Frontend expects { tokens: { accessToken, refreshToken } }
	pkg.WriteJSON(w, http.StatusOK, map[string]any{
		"tokens": authTokens{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
		},
	})
}

// Logout revokes the Socrate refresh token (best effort).
// POST /auth/logout  { refreshToken }
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		pkg.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if h.tokens != nil && req.RefreshToken != "" {
		if err := h.tokens.RevokeToken(r.Context(), req.RefreshToken); err != nil {
			logrus.WithError(err).Warn("auth: refresh-token revocation failed")
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
