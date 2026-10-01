package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ovander/backendkit/socrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTokens records what the /auth routes ask Socrate for.
type fakeTokens struct {
	code, redirectURI, verifier string
	refreshed, revoked          string
	err                         error
}

func (f *fakeTokens) ExchangeCode(_ context.Context, code, redirectURI, verifier string) (*socrate.TokenSet, error) {
	f.code, f.redirectURI, f.verifier = code, redirectURI, verifier
	if f.err != nil {
		return nil, f.err
	}
	return &socrate.TokenSet{AccessToken: "at-1", RefreshToken: "rt-1"}, nil
}

func (f *fakeTokens) RefreshToken(_ context.Context, rt string) (*socrate.TokenSet, error) {
	f.refreshed = rt
	if f.err != nil {
		return nil, f.err
	}
	return &socrate.TokenSet{AccessToken: "at-2", RefreshToken: "rt-2"}, nil
}

func (f *fakeTokens) RevokeToken(_ context.Context, token string) error {
	f.revoked = token
	return f.err
}

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	return rec
}

func TestAuthCallbackExchangesThroughSocrateClient(t *testing.T) {
	f := &fakeTokens{}
	h := NewAuthHandler(f, "https://parashift.example/callback")

	rec := post(h.Callback, `{"code":"c1","codeVerifier":"v1"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "c1", f.code)
	assert.Equal(t, "v1", f.verifier)
	assert.Equal(t, "https://parashift.example/callback", f.redirectURI, "falls back to SOCRATE_REDIRECT_URL")
	var got authTokens
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, authTokens{AccessToken: "at-1", RefreshToken: "rt-1"}, got)
}

func TestAuthCallbackHidesSocrateError(t *testing.T) {
	h := NewAuthHandler(&fakeTokens{err: &socrate.OAuthError{Code: "invalid_grant", Description: "secret detail"}}, "")

	rec := post(h.Callback, `{"code":"c1","codeVerifier":"v1"}`)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.NotContains(t, rec.Body.String(), "secret detail")
}

func TestAuthRefreshKeepsResponseShape(t *testing.T) {
	f := &fakeTokens{}
	h := NewAuthHandler(f, "")

	rec := post(h.Refresh, `{"refreshToken":"rt-1"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "rt-1", f.refreshed)
	var got struct {
		Tokens authTokens `json:"tokens"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, authTokens{AccessToken: "at-2", RefreshToken: "rt-2"}, got.Tokens)
}

func TestAuthLogoutRevokesAndIgnoresFailure(t *testing.T) {
	f := &fakeTokens{err: errors.New("socrate down")}
	h := NewAuthHandler(f, "")

	rec := post(h.Logout, `{"refreshToken":"rt-1"}`)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "rt-1", f.revoked)
}

func TestAuthRoutesWithoutSocrate(t *testing.T) {
	h := NewAuthHandler(nil, "")

	assert.Equal(t, http.StatusServiceUnavailable, post(h.Callback, `{"code":"c","codeVerifier":"v"}`).Code)
	assert.Equal(t, http.StatusServiceUnavailable, post(h.Refresh, `{"refreshToken":"r"}`).Code)
	assert.Equal(t, http.StatusNoContent, post(h.Logout, `{"refreshToken":"r"}`).Code)
}
