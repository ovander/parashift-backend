package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/sirupsen/logrus"
)

// AuthHandler handles OAuth2 token exchange with Socrate.
// It follows the same pattern as Ascenda/KerPlan:
//   - /auth/callback  → exchange code for tokens (lean, no DB)
//   - /auth/refresh   → rotate tokens
//   - /auth/logout    → revoke refresh token
//
// User enrichment (ParaShift role, store_id) is the responsibility of GET /me,
// which is called by the frontend immediately after the callback.
type AuthHandler struct {
	cfg    config.SocrateConfig
	client *http.Client
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(cfg config.SocrateConfig) *AuthHandler {
	return &AuthHandler{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
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

// socrateTokenResponse is what Socrate's /oauth/token endpoint returns.
type socrateTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// authTokens is what the frontend expects from /auth/callback and /auth/refresh.
type authTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// Callback exchanges a PKCE authorization code for a Socrate token pair.
// POST /auth/callback  { code, codeVerifier, redirectUri }
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
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
		redirectURI = h.cfg.RedirectURL
	}

	tokens, err := h.exchangeCode(req.Code, req.CodeVerifier, redirectURI)
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
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		pkg.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "refreshToken is required"})
		return
	}

	tokens, err := h.refreshToken(req.RefreshToken)
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
	_ = h.revokeToken(req.RefreshToken)
	w.WriteHeader(http.StatusNoContent)
}

// ── Socrate helpers ───────────────────────────────────────────────────────────

func (h *AuthHandler) exchangeCode(code, codeVerifier, redirectURI string) (*socrateTokenResponse, error) {
	return h.postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {h.cfg.ClientID},
		"client_secret": {h.cfg.ClientSecret},
		"code_verifier": {codeVerifier},
	})
}

func (h *AuthHandler) refreshToken(token string) (*socrateTokenResponse, error) {
	return h.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {token},
		"client_id":     {h.cfg.ClientID},
		"client_secret": {h.cfg.ClientSecret},
	})
}

func (h *AuthHandler) revokeToken(token string) error {
	endpoint := strings.TrimRight(h.cfg.BaseURL, "/") + "/oauth/revoke"
	resp, err := h.client.PostForm(endpoint, url.Values{
		"token":         {token},
		"client_id":     {h.cfg.ClientID},
		"client_secret": {h.cfg.ClientSecret},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (h *AuthHandler) postToken(form url.Values) (*socrateTokenResponse, error) {
	endpoint := strings.TrimRight(h.cfg.BaseURL, "/") + "/oauth/token"
	resp, err := h.client.PostForm(endpoint, form)
	if err != nil {
		return nil, fmt.Errorf("request to Socrate failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("socrate returned %d: %s", resp.StatusCode, string(body))
	}

	var tokens socrateTokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse Socrate response: %w", err)
	}
	return &tokens, nil
}
