package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prodEnv is a complete, valid production environment (placeholder values).
func prodEnv(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"ENV":                   "production",
		"DATABASE_URL":          "postgres://u:p@localhost:5432/parashift?sslmode=require",
		"ALLOWED_ORIGINS":       "https://parashift.example",
		"SOCRATE_BASE_URL":      "https://socrate.example",
		"SOCRATE_JWKS_URL":      "https://socrate.example/.well-known/jwks.json",
		"SOCRATE_ADMIN_URL":     "http://127.0.0.1:18082",
		"SOCRATE_CLIENT_ID":     "client",
		"SOCRATE_CLIENT_SECRET": "secret",
		"SOCRATE_APP_ID":        "7",
		"SOCRATE_ISSUER":        "",
		"BIND_ADDR":             "",
	} {
		t.Setenv(k, v)
	}
}

func validateErr(t *testing.T) string {
	t.Helper()
	err := Load().Validate()
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestValidProductionConfig(t *testing.T) {
	prodEnv(t)
	assert.Empty(t, validateErr(t))
}

func TestProductionRequiresAppIDAndAdminURL(t *testing.T) {
	prodEnv(t)
	t.Setenv("SOCRATE_APP_ID", "")
	t.Setenv("SOCRATE_ADMIN_URL", "")
	msg := validateErr(t)
	assert.Contains(t, msg, "SOCRATE_APP_ID is required in production")
	assert.Contains(t, msg, "SOCRATE_ADMIN_URL is required in production")
}

func TestAppIDMustBeNumeric(t *testing.T) {
	prodEnv(t)
	t.Setenv("SOCRATE_APP_ID", "parashift")
	assert.Contains(t, validateErr(t), "SOCRATE_APP_ID must be a positive integer")
}

func TestBaseURLWithoutTrailingSlash(t *testing.T) {
	prodEnv(t)
	t.Setenv("SOCRATE_BASE_URL", "https://socrate.example/")
	assert.Contains(t, validateErr(t), "SOCRATE_BASE_URL must not end with a slash")
}

func TestIssuerDefaultsToBaseURLAndMustMatch(t *testing.T) {
	prodEnv(t)
	assert.Equal(t, "https://socrate.example", Load().JWKS.Issuer)

	t.Setenv("SOCRATE_ISSUER", "https://other.example")
	assert.Contains(t, validateErr(t), "SOCRATE_ISSUER must equal SOCRATE_BASE_URL")
}

func TestENVIsRequired(t *testing.T) {
	prodEnv(t)
	t.Setenv("ENV", "")
	assert.Contains(t, validateErr(t), "ENV is required")
	t.Setenv("ENV", "prod")
	assert.Contains(t, validateErr(t), `ENV="prod" is not one of`)
}

func TestDevelopmentOnlyChecksURLShapes(t *testing.T) {
	prodEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SOCRATE_APP_ID", "")
	t.Setenv("SOCRATE_ADMIN_URL", "")
	t.Setenv("SOCRATE_BASE_URL", "http://localhost:9000")
	t.Setenv("SOCRATE_JWKS_URL", "http://localhost:9000/.well-known/jwks.json")
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5181")
	assert.Empty(t, validateErr(t))
}

func TestListenAddr(t *testing.T) {
	prodEnv(t)
	t.Setenv("PORT", "4000")
	assert.Equal(t, "127.0.0.1:4000", Load().ListenAddr(), "production binds loopback by default")

	t.Setenv("BIND_ADDR", "0.0.0.0")
	assert.Equal(t, "0.0.0.0:4000", Load().ListenAddr())

	t.Setenv("ENV", "development")
	t.Setenv("BIND_ADDR", "")
	assert.Equal(t, ":4000", Load().ListenAddr())
}

func TestDebugRouteOnlyInDevelopment(t *testing.T) {
	for env, want := range map[string]bool{"development": true, "production": false, "test": false, "": false} {
		c := &Config{Env: env}
		require.Equal(t, want, c.IsDevelopment(), "ENV=%q", env)
	}
}
