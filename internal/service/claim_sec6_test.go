package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// claimFixture builds an EmployeeService whose claim-token lookup returns emp,
// and captures whether/what Update persisted.
func claimFixture(emp *model.Employee) (*service.EmployeeService, *bool, **model.Employee) {
	updateCalled := false
	var updated *model.Employee
	repo := &testutil.MockEmployeeRepo{
		GetByClaimTokenFn: func(_ context.Context, _ string) (*model.Employee, error) { return emp, nil },
		UpdateFn: func(_ context.Context, e *model.Employee) error {
			updateCalled = true
			updated = e
			return nil
		},
	}
	svc := service.NewEmployeeService(repo, &testutil.MockContractRepo{}, newTestEmitter(), newTestLogger())
	return svc, &updateCalled, &updated
}

func issuedEmp(email string, issued time.Time) *model.Employee {
	tok := "tok-" + uuid.NewString()
	e := testutil.NewEmployee(uuid.New())
	e.Email = email
	e.ClaimToken = &tok
	e.ClaimTokenIssuedAt = &issued
	return e
}

// SEC-6: a valid, in-TTL token with a matching email binds the sub and clears the token.
func TestClaim_Success_BindsAndClears(t *testing.T) {
	emp := issuedEmp("Marie@Example.com", time.Now().Add(-1*time.Hour))
	svc, called, updated := claimFixture(emp)

	got, err := svc.ClaimByToken(context.Background(), *emp.ClaimToken, "socrate-sub-1", "marie@example.com")
	require.NoError(t, err)
	assert.Equal(t, "socrate-sub-1", got.AuthID)
	assert.Nil(t, got.ClaimToken, "token must be cleared after claim")
	assert.True(t, *called)
	assert.Nil(t, (*updated).ClaimTokenIssuedAt, "issued-at must be cleared")
}

// SEC-6: an expired token is rejected and nothing is bound.
func TestClaim_Rejected_WhenExpired(t *testing.T) {
	emp := issuedEmp("marie@example.com", time.Now().Add(-100*time.Hour)) // > 72h
	svc, called, _ := claimFixture(emp)

	_, err := svc.ClaimByToken(context.Background(), *emp.ClaimToken, "sub", "marie@example.com")
	require.Error(t, err)
	assert.False(t, *called, "expired token must not bind the account")
}

// SEC-6: a mismatched email is rejected (leaked link claimed by another account).
func TestClaim_Rejected_OnEmailMismatch(t *testing.T) {
	emp := issuedEmp("invited@example.com", time.Now().Add(-1*time.Hour))
	svc, called, _ := claimFixture(emp)

	_, err := svc.ClaimByToken(context.Background(), *emp.ClaimToken, "sub", "attacker@evil.com")
	require.Error(t, err)
	assert.False(t, *called, "email mismatch must not bind the account")
}

// SEC-6: matching email is case/space-insensitive.
func TestClaim_Success_EmailCaseInsensitive(t *testing.T) {
	emp := issuedEmp("Invited@Example.com", time.Now().Add(-1*time.Hour))
	svc, called, _ := claimFixture(emp)

	_, err := svc.ClaimByToken(context.Background(), *emp.ClaimToken, "sub", "  invited@example.com ")
	require.NoError(t, err)
	assert.True(t, *called)
}

// SEC-6: when the caller email is unknown (access token without email claim) the
// claim proceeds (best-effort) — TTL still applies.
func TestClaim_Success_WhenCallerEmailUnknown(t *testing.T) {
	emp := issuedEmp("invited@example.com", time.Now().Add(-1*time.Hour))
	svc, called, _ := claimFixture(emp)

	_, err := svc.ClaimByToken(context.Background(), *emp.ClaimToken, "sub", "")
	require.NoError(t, err)
	assert.True(t, *called)
}

// SEC-6: legacy rows without claim_token_issued_at fall back to created_at for TTL.
func TestClaim_LegacyRow_FallsBackToCreatedAt(t *testing.T) {
	emp := testutil.NewEmployee(uuid.New())
	tok := "legacy-tok"
	emp.Email = "x@example.com"
	emp.ClaimToken = &tok
	emp.ClaimTokenIssuedAt = nil
	emp.CreatedAt = time.Now().Add(-200 * time.Hour) // old → expired via fallback
	svc, called, _ := claimFixture(emp)

	_, err := svc.ClaimByToken(context.Background(), tok, "sub", "x@example.com")
	require.Error(t, err, "legacy row older than TTL must be rejected")
	assert.False(t, *called)
}
