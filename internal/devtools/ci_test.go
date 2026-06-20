// Package devtools holds repo-hygiene guard tests (no runtime code).
package devtools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoFile resolves a path relative to the repository root.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	abs, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", "..", rel))
	require.NoError(t, err)
	return abs
}

// TestCIWorkflowPresent guards the DX-1 quality gate: the CI workflow must
// exist and exercise build, vet, lint, race tests, and coverage. Prevents the
// gate from being silently deleted or hollowed out.
func TestCIWorkflowPresent(t *testing.T) {
	b, err := os.ReadFile(repoFile(t, ".github/workflows/ci.yml"))
	require.NoError(t, err, "CI workflow must exist")
	ci := string(b)

	for _, want := range []string{
		"go build ./...",
		"go vet ./...",
		"golangci-lint",
		"go test -race",
		"-coverprofile=coverage.out",
		"pull_request",
	} {
		assert.Contains(t, ci, want, "CI workflow must run %q", want)
	}
}

// TestGolangciConfigPresent guards the DX-2 lint config (CI depends on it).
func TestGolangciConfigPresent(t *testing.T) {
	_, err := os.Stat(repoFile(t, ".golangci.yml"))
	require.NoError(t, err, ".golangci.yml must exist for the CI lint step")
}
