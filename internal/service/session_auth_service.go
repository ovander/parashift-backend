package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
)

// SessionTokens is the part of *socrate.Client the BFF sign-in uses on the
// token endpoints: the authorization-code exchange and the revocation at
// sign-out (Socrate has no end-session endpoint).
type SessionTokens interface {
	ExchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*socrate.TokenSet, error)
	RevokeToken(ctx context.Context, token string) error
}

// SessionProfiles reads the signed-in user's Socrate profile with the access
// token in ctx (socrate.WithJWT). Satisfied by *socrate.Client (GET
// /api/profile), which returns the e-mail address and whether it is verified.
type SessionProfiles interface {
	GetProfile(ctx context.Context) (*socrate.FullProfile, error)
}

// SessionIdentity is who signed in, as the BFF keeps it on the server-side
// session and shows it to the SPA. It holds no token. Email is set only when
// Socrate has verified the address, so the invite claim can rely on it.
type SessionIdentity struct {
	Sub   string
	Email string
	Name  string
}

// SessionAuthService runs the Backend-for-Frontend sign-in: it redeems an
// authorization code for a token set and reads the user's profile. The tokens
// it returns go on the server-side session only, never to the browser.
type SessionAuthService struct {
	tokens   SessionTokens
	profiles SessionProfiles
	logger   *logrus.Entry
}

// NewSessionAuthService creates a SessionAuthService.
func NewSessionAuthService(tokens SessionTokens, profiles SessionProfiles, logger *logrus.Entry) *SessionAuthService {
	return &SessionAuthService{tokens: tokens, profiles: profiles, logger: logger}
}

// SignIn exchanges an authorization code and its PKCE verifier, then reads
// the user's profile. Any failure is an error; the handler sends the browser
// back to the sign-in page without saying why.
func (s *SessionAuthService) SignIn(ctx context.Context, code, redirectURI, verifier string) (*socrate.TokenSet, SessionIdentity, error) {
	ts, err := s.tokens.ExchangeCode(ctx, code, redirectURI, verifier)
	if err != nil {
		return nil, SessionIdentity{}, err
	}
	if ts == nil || ts.AccessToken == "" {
		return nil, SessionIdentity{}, apierror.Internal("sign-in failed").WithKey("errors.unknown")
	}
	if ts.RefreshToken == "" {
		// Without a refresh token the session ends at the first expiry.
		s.logger.Warn("bff: Socrate returned no refresh token; the session ends when the access token expires")
	}
	p, err := s.profiles.GetProfile(socrate.WithJWT(ctx, ts.AccessToken))
	if err != nil || p == nil || p.ID == 0 {
		s.logger.WithError(err).Error("bff: profile read failed after sign-in")
		return nil, SessionIdentity{}, apierror.Internal("sign-in failed").WithKey("errors.unknown")
	}
	id := SessionIdentity{
		// Socrate's sub is its user's numeric ID, the same value jwtauth reads.
		Sub:  strconv.FormatUint(uint64(p.ID), 10),
		Name: strings.TrimSpace(p.Name),
	}
	if p.IsVerified {
		id.Email = strings.TrimSpace(p.Email)
	}
	return ts, id, nil
}

// SignOut revokes the session's refresh token, which ends its rotation chain
// at Socrate. A session without one has nothing to revoke.
func (s *SessionAuthService) SignOut(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.tokens.RevokeToken(ctx, refreshToken)
}
