package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ovander/backendkit/jwtauth"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRevSvc(repo *testutil.MockTokenRevocationRepo) *RevocationService {
	return NewRevocationService(repo, logrus.NewEntry(logrus.New()))
}

func claimsFor(sub string, issuedAt time.Time) *jwtauth.SocrateClaims {
	return &jwtauth.SocrateClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  sub,
			IssuedAt: jwt.NewNumericDate(issuedAt),
		},
	}
}

// No revocation record → the token is accepted.
func TestRevocation_AllowsWhenNoFloor(t *testing.T) {
	svc := newRevSvc(&testutil.MockTokenRevocationRepo{})
	err := svc.CheckToken(context.Background(), claimsFor("user-1", time.Now()))
	assert.NoError(t, err)
}

// A token issued before the floor is rejected; a fresh token issued after passes.
func TestRevocation_RejectsTokenIssuedBeforeFloor(t *testing.T) {
	floor := time.Now()
	repo := &testutil.MockTokenRevocationRepo{
		GetBySubFn: func(_ context.Context, sub string) (*model.TokenRevocation, error) {
			return &model.TokenRevocation{Sub: sub, RevokedAfter: floor}, nil
		},
	}
	svc := newRevSvc(repo)

	// Issued one minute before the floor → revoked.
	err := svc.CheckToken(context.Background(), claimsFor("user-1", floor.Add(-time.Minute)))
	require.Error(t, err)

	// Issued one minute after the floor → still valid.
	err = svc.CheckToken(context.Background(), claimsFor("user-1", floor.Add(time.Minute)))
	assert.NoError(t, err)
}

// A token exactly at the floor is rejected (fail closed on the boundary).
func TestRevocation_RejectsTokenAtFloorBoundary(t *testing.T) {
	floor := time.Now()
	repo := &testutil.MockTokenRevocationRepo{
		GetBySubFn: func(_ context.Context, sub string) (*model.TokenRevocation, error) {
			return &model.TokenRevocation{Sub: sub, RevokedAfter: floor}, nil
		},
	}
	err := newRevSvc(repo).CheckToken(context.Background(), claimsFor("user-1", floor))
	assert.Error(t, err)
}

// Missing iat while a floor exists → fail closed.
func TestRevocation_RejectsMissingIatWithFloor(t *testing.T) {
	repo := &testutil.MockTokenRevocationRepo{
		GetBySubFn: func(_ context.Context, sub string) (*model.TokenRevocation, error) {
			return &model.TokenRevocation{Sub: sub, RevokedAfter: time.Now()}, nil
		},
	}
	claims := &jwtauth.SocrateClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: "user-1"}}
	assert.Error(t, newRevSvc(repo).CheckToken(context.Background(), claims))
}

// Empty subject and nil claims are no-ops (nothing to check).
func TestRevocation_NoSubjectIsNoop(t *testing.T) {
	svc := newRevSvc(&testutil.MockTokenRevocationRepo{})
	assert.NoError(t, svc.CheckToken(context.Background(), claimsFor("", time.Now())))
	assert.NoError(t, svc.CheckToken(context.Background(), nil))
}

// Availability over strictness: a store error does not lock the user out.
func TestRevocation_FailsOpenOnRepoError(t *testing.T) {
	repo := &testutil.MockTokenRevocationRepo{
		GetBySubFn: func(_ context.Context, _ string) (*model.TokenRevocation, error) {
			return nil, errors.New("db down")
		},
	}
	assert.NoError(t, newRevSvc(repo).CheckToken(context.Background(), claimsFor("user-1", time.Now())))
}

// RevokeSessions writes a floor and takes effect immediately on this instance,
// without waiting for the cache TTL.
func TestRevocation_RevokeSessionsTakesEffectImmediately(t *testing.T) {
	var stored atomic.Value // time.Time
	repo := &testutil.MockTokenRevocationRepo{
		UpsertFn: func(_ context.Context, _ string, revokedAfter time.Time) error {
			stored.Store(revokedAfter)
			return nil
		},
		GetBySubFn: func(_ context.Context, _ string) (*model.TokenRevocation, error) {
			// Returns "no floor". If the post-revoke check fell through to the repo
			// instead of the freshly-written cache, it would wrongly pass — so the
			// final assertion proves the cache was updated by RevokeSessions.
			return nil, nil
		},
	}
	svc := newRevSvc(repo)
	ctx := context.Background()

	// A token issued "now" is valid before the revoke.
	tok := time.Now()
	require.NoError(t, svc.CheckToken(ctx, claimsFor("user-1", tok)))

	require.NoError(t, svc.RevokeSessions(ctx, "user-1"))
	require.NotNil(t, stored.Load(), "floor must be persisted")

	// The same token is now rejected via the locally-updated cache.
	assert.Error(t, svc.CheckToken(ctx, claimsFor("user-1", tok)))
}

// The floor is cached: repeated checks for the same subject hit the repo once
// within the TTL.
func TestRevocation_CachesFloorLookups(t *testing.T) {
	var calls int32
	repo := &testutil.MockTokenRevocationRepo{
		GetBySubFn: func(_ context.Context, _ string) (*model.TokenRevocation, error) {
			atomic.AddInt32(&calls, 1)
			return nil, nil
		},
	}
	svc := newRevSvc(repo)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		require.NoError(t, svc.CheckToken(ctx, claimsFor("user-1", time.Now())))
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "lookups should be cached within the TTL")
}
