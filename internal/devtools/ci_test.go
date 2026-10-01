// Package devtools holds repo-hygiene guard tests (no runtime code).
package devtools

import (
	"os"
	"os/exec"
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
		"gofmt -l",
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

// gitIgnored reports whether git treats path as ignored (check-ignore exits 0
// when ignored, 1 when not). Skips if git or the work tree is unavailable.
func gitIgnored(t *testing.T, path string) bool {
	t.Helper()
	root := repoFile(t, ".")
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cmd := exec.Command("git", "-C", root, "check-ignore", "-q", path)
	err := cmd.Run()
	if err == nil {
		return true // exit 0 → ignored
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false // exit 1 → not ignored
	}
	t.Skipf("git check-ignore unavailable here: %v", err)
	return false
}

// TestGitignoreAnchorsBinaries guards the fix for the over-broad server/app
// patterns: real source under cmd/server must NOT be ignored, while the
// repo-root binaries still are.
func TestGitignoreAnchorsBinaries(t *testing.T) {
	assert.False(t, gitIgnored(t, "cmd/server/main.go"),
		"cmd/server source must not be git-ignored")
	assert.False(t, gitIgnored(t, "cmd/server/anything_test.go"),
		"new files under cmd/server must not be git-ignored")
	assert.True(t, gitIgnored(t, "server"),
		"the repo-root server binary should still be ignored")
	assert.True(t, gitIgnored(t, "app"),
		"the repo-root app binary should still be ignored")
}
