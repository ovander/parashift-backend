package service

import (
	"context"
	"sync"
	"time"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/jwtauth"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// revocationCacheTTL bounds how long a revocation floor is cached in-process.
// It is also the worst-case propagation delay for a revocation issued on a
// different replica: that replica writes the floor to the database immediately,
// but other replicas only observe it once their cached entry expires.
const revocationCacheTTL = 15 * time.Second

// RevocationService enforces and manages instant session revocation. It backs
// jwtauth.WithRevocationCheck: a token is rejected when it was issued at or
// before the subject's revocation floor (RevokedAfter). Revoking a subject's
// sessions raises that floor to "now", so every token minted earlier — across
// the fleet — stops being accepted, while a fresh login (later iat) works again.
type RevocationService struct {
	repo   repo.TokenRevocationRepository
	logger *logrus.Entry
	ttl    time.Duration
	now    func() time.Time

	mu    sync.RWMutex
	cache map[string]revEntry
}

// revEntry caches a subject's floor. found=false caches the common "no
// revocation" case so unrevoked users don't hit the DB on every request.
type revEntry struct {
	revokedAfter time.Time
	found        bool
	expiry       time.Time
}

// NewRevocationService creates a RevocationService.
func NewRevocationService(r repo.TokenRevocationRepository, logger *logrus.Entry) *RevocationService {
	return &RevocationService{
		repo:   r,
		logger: logger,
		ttl:    revocationCacheTTL,
		now:    time.Now,
		cache:  make(map[string]revEntry),
	}
}

// CheckToken is the jwtauth.RevocationChecker. It runs after signature, issuer,
// audience and expiry validation. It returns a non-nil error (→ 401) when the
// token predates the subject's revocation floor.
//
// Availability over strictness on infrastructure faults: if the floor cannot be
// loaded (DB error) the request is allowed and the error is logged, so a
// revocation-store outage cannot lock every user out. A token that lacks an iat
// while a floor exists is rejected (fail closed), since its freshness can't be
// proven.
func (s *RevocationService) CheckToken(ctx context.Context, claims *jwtauth.SocrateClaims) error {
	if claims == nil || claims.Subject == "" {
		return nil
	}
	entry, err := s.floor(ctx, claims.Subject)
	if err != nil {
		s.logger.WithError(err).WithField("sub", claims.Subject).
			Warn("revocation check skipped: could not load floor")
		return nil
	}
	if !entry.found {
		return nil
	}
	if claims.IssuedAt == nil {
		return apierror.Unauthorized("token revoked").WithKey("errors.tokenRevoked")
	}
	// Reject tokens issued at or before the floor (fail closed on the boundary).
	if !claims.IssuedAt.Time.After(entry.revokedAfter) {
		return apierror.Unauthorized("token revoked").WithKey("errors.tokenRevoked")
	}
	return nil
}

// RevokeSessions invalidates every access token currently held by sub by raising
// its revocation floor to now. It updates the local cache immediately so this
// replica enforces without waiting for the cache TTL; other replicas converge
// within revocationCacheTTL.
func (s *RevocationService) RevokeSessions(ctx context.Context, sub string) error {
	if sub == "" {
		return apierror.BadRequest("missing subject").WithKey("errors.invalidInput")
	}
	at := s.now()
	if err := s.repo.Upsert(ctx, sub, at); err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[sub] = revEntry{revokedAfter: at, found: true, expiry: at.Add(s.ttl)}
	s.mu.Unlock()
	s.logger.WithField("sub", sub).Info("revoked all active sessions for subject")
	return nil
}

// floor returns the (possibly cached) revocation floor for sub.
func (s *RevocationService) floor(ctx context.Context, sub string) (revEntry, error) {
	now := s.now()

	s.mu.RLock()
	if e, ok := s.cache[sub]; ok && now.Before(e.expiry) {
		s.mu.RUnlock()
		return e, nil
	}
	s.mu.RUnlock()

	rec, err := s.repo.GetBySub(ctx, sub)
	if err != nil {
		return revEntry{}, err
	}

	e := revEntry{expiry: now.Add(s.ttl)}
	if rec != nil {
		e.found = true
		e.revokedAfter = rec.RevokedAfter
	}

	s.mu.Lock()
	s.cache[sub] = e
	s.mu.Unlock()
	return e, nil
}
