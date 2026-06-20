// Package dbmigrations hosts DB-free integrity tests for the SQL migration set.
// (It has no non-test sources; cmd/server is git-ignored via the "server"
// pattern, so the migration tests live here instead.)
package dbmigrations

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// migrationsDir resolves the repo's migrations directory relative to this file.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	abs, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations"))
	require.NoError(t, err)
	return abs
}

// TestMigrations_FormValidSequence verifies the whole migration set is a valid,
// parseable golang-migrate sequence (each version has a matching up+down and the
// chain walks without error). DB-free guardrail enforced in CI.
func TestMigrations_FormValidSequence(t *testing.T) {
	src, err := (&file.File{}).Open("file://" + migrationsDir(t))
	require.NoError(t, err, "migration directory must be a valid golang-migrate source")

	version, err := src.First()
	require.NoError(t, err, "there must be at least one migration")

	count := 1
	for {
		_, _, err := src.ReadUp(version)
		require.NoError(t, err, "missing/unreadable up migration for version %d", version)
		_, _, err = src.ReadDown(version)
		require.NoError(t, err, "missing/unreadable down migration for version %d", version)

		next, err := src.Next(version)
		if err != nil {
			break
		}
		version = next
		count++
	}
	assert.GreaterOrEqual(t, count, 33, "expected the full migration history to be present")
}

// TestMigration33_ScrubsLeakedTokens guards the SEC-4 remediation: the migration
// must scrub the audit payloads and rotate outstanding invite tokens.
func TestMigration33_ScrubsLeakedTokens(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(migrationsDir(t), "000033_scrub_leaked_invite_tokens.up.sql"))
	require.NoError(t, err, "SEC-4 migration must exist")
	sql := strings.ToLower(string(b))

	assert.Contains(t, sql, "update audit_logs", "must scrub the audit table")
	assert.Contains(t, sql, "claimtoken", "must strip the ClaimToken key")
	assert.Contains(t, sql, "authid", "must strip the AuthID key")
	assert.Contains(t, sql, "update employees", "must rotate outstanding invite tokens")
	assert.Contains(t, sql, "claim_token = gen_random_uuid()", "must rotate to a fresh random token")
}
