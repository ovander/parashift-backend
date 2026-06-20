package event

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SEC-2 regression: the audit trail must never persist invite tokens or auth
// subjects. sanitizeForAudit projects Employee payloads onto an allow-list.
func TestSanitizeForAudit_EmployeeExcludesSecrets(t *testing.T) {
	token := "super-secret-claim-token"
	emp := &model.Employee{
		Name:       "Marie Dubois",
		Position:   "manager",
		JobRole:    "pharmacist",
		Email:      "marie@example.com",
		AuthID:     "socrate-sub-secret",
		ClaimToken: &token,
	}
	emp.ID = uuid.New()
	emp.TenantID = uuid.New()

	out, err := json.Marshal(sanitizeForAudit(emp))
	require.NoError(t, err)
	s := string(out)

	assert.NotContains(t, s, token, "claim token must not appear in audit payload")
	assert.NotContains(t, s, "socrate-sub-secret", "auth_id must not appear in audit payload")
	assert.NotContains(t, s, "claim_token")
	assert.NotContains(t, s, "auth_id")
	// Useful, non-sensitive fields are retained.
	assert.Contains(t, s, "Marie Dubois")
	assert.Contains(t, s, "manager")
}

// Non-employee payloads pass through unchanged.
func TestSanitizeForAudit_PassesThroughOtherPayloads(t *testing.T) {
	payload := map[string]any{"action": "reset_week"}
	assert.Equal(t, payload, sanitizeForAudit(payload))
}

// Defense-in-depth: marshaling a raw Employee (e.g. anywhere in the codebase)
// must also drop the secret fields thanks to the json:"-" tags.
func TestEmployee_JSONOmitsSecrets(t *testing.T) {
	token := "another-secret"
	emp := &model.Employee{AuthID: "sub-xyz", ClaimToken: &token, Name: "Bob"}
	out, err := json.Marshal(emp)
	require.NoError(t, err)
	s := strings.ToLower(string(out))
	assert.NotContains(t, s, "auth_id")
	assert.NotContains(t, s, "claim_token")
	assert.NotContains(t, s, "sub-xyz")
	assert.NotContains(t, s, "another-secret")
}
