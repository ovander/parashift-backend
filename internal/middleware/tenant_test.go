package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
)

// fakeEmployees implements the EmployeeRepository calls TenantMiddleware
// makes; any other call panics on the nil embedded interface.
type fakeEmployees struct {
	repo.EmployeeRepository
	byAuthID map[string]*model.Employee
	unlinked map[string]*model.Employee // by e-mail, auth_id empty
	updated  []*model.Employee
}

func (f *fakeEmployees) GetByAuthID(_ context.Context, sub string) (*model.Employee, error) {
	return f.byAuthID[sub], nil
}

func (f *fakeEmployees) GetByEmail(_ context.Context, email string) (*model.Employee, error) {
	return f.unlinked[email], nil
}

func (f *fakeEmployees) Update(_ context.Context, e *model.Employee) error {
	f.updated = append(f.updated, e)
	return nil
}

type fakeProfiles struct {
	profile *socrate.FullProfile
	err     error
	calls   int
}

func (f *fakeProfiles) GetProfile(context.Context) (*socrate.FullProfile, error) {
	f.calls++
	return f.profile, f.err
}

func newUnlinked() (*fakeEmployees, *model.Employee) {
	emp := &model.Employee{Email: "marie@example.com", Position: "employee"}
	emp.ID = uuid.New()
	emp.TenantID = uuid.New()
	return &fakeEmployees{
		byAuthID: map[string]*model.Employee{},
		unlinked: map[string]*model.Employee{"marie@example.com": emp},
	}, emp
}

// serve runs one request for sub through the middleware and reports the
// status and the tenant the next handler saw.
func serve(m *TenantMiddleware, sub string) (int, uuid.UUID) {
	var tenant uuid.UUID
	h := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant = ctxutil.GetTenantID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req = req.WithContext(ctxutil.WithUserSub(req.Context(), sub))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, tenant
}

func quietLogger() *logrus.Entry {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return logrus.NewEntry(l)
}

func TestTenantAutoLinkVerifiedEmail(t *testing.T) {
	emps, emp := newUnlinked()
	profiles := &fakeProfiles{profile: &socrate.FullProfile{Email: "marie@example.com", IsVerified: true}}
	m := NewTenantMiddleware(emps, quietLogger(), profiles)

	code, tenant := serve(m, "42")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, emp.TenantID, tenant)
	if assert.Len(t, emps.updated, 1) {
		assert.Equal(t, "42", emps.updated[0].AuthID)
	}
}

func TestTenantAutoLinkRefusesUnverifiedEmail(t *testing.T) {
	emps, _ := newUnlinked()
	profiles := &fakeProfiles{profile: &socrate.FullProfile{Email: "marie@example.com", IsVerified: false}}
	m := NewTenantMiddleware(emps, quietLogger(), profiles)

	code, _ := serve(m, "42")

	assert.Equal(t, http.StatusNotFound, code)
	assert.Empty(t, emps.updated, "an unverified address must never link an employee")
}

func TestTenantAutoLinkSkippedOnProfileError(t *testing.T) {
	emps, _ := newUnlinked()
	profiles := &fakeProfiles{err: errors.New("socrate down")}
	m := NewTenantMiddleware(emps, quietLogger(), profiles)

	code, _ := serve(m, "42")

	assert.Equal(t, http.StatusNotFound, code)
	assert.Empty(t, emps.updated)
}

func TestTenantAutoLinkDisabledWithoutProfileReader(t *testing.T) {
	emps, _ := newUnlinked()
	m := NewTenantMiddleware(emps, quietLogger(), nil)

	code, _ := serve(m, "42")

	assert.Equal(t, http.StatusNotFound, code)
	assert.Empty(t, emps.updated)
}

func TestTenantLinkedEmployeeNeedsNoProfile(t *testing.T) {
	emps, emp := newUnlinked()
	emp.AuthID = "42"
	emps.byAuthID["42"] = emp
	profiles := &fakeProfiles{}
	m := NewTenantMiddleware(emps, quietLogger(), profiles)

	code, tenant := serve(m, "42")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, emp.TenantID, tenant)
	assert.Zero(t, profiles.calls, "a linked employee must not cost a Socrate call")
}
