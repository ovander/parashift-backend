package pkg_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logrus.SetOutput(io.Discard) // silence the server-side error log during tests
	m.Run()
}

// SEC-8: a non-AppError must not leak internal detail to the client.
func TestWriteError_SanitizesInternalError(t *testing.T) {
	rec := httptest.NewRecorder()
	leak := "pq: duplicate key value violates unique constraint \"employees_secret_idx\""
	pkg.WriteError(rec, errors.New(leak))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "duplicate key", "internal error detail must not reach the client")
	assert.NotContains(t, body, "employees_secret_idx")
	assert.NotContains(t, body, "pq:")
	assert.Contains(t, strings.ToLower(body), "internal server error")
}

// AppErrors (with an i18n key) are preserved verbatim — they're caller-authored
// and safe to surface.
func TestWriteError_PreservesAppError(t *testing.T) {
	rec := httptest.NewRecorder()
	pkg.WriteError(rec, apierror.BadRequest("name is required").WithKey("errors.invalidInput"))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "errors.invalidInput")
}

// CR-5 guard: an AppError without an i18n key panics in development and tests,
// but in production it must be answered with its own status, not turned into a
// panic and a 500. The guard used to read APP_ENV, which nothing sets.
func TestWriteError_MissingKeyGuardFollowsENV(t *testing.T) {
	keyless := func() error { return apierror.BadRequest("no key") }

	t.Setenv("ENV", "development")
	assert.Panics(t, func() { pkg.WriteError(httptest.NewRecorder(), keyless()) })

	t.Setenv("ENV", "production")
	rec := httptest.NewRecorder()
	assert.NotPanics(t, func() { pkg.WriteError(rec, keyless()) })
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
