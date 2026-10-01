package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ovander/backendkit/ctxutil"
	"github.com/stretchr/testify/assert"
)

const parashiftClient = "parashift-client"

// roleSeen runs AppRole over a request whose context holds what jwtauth puts
// there (the top-level role and app_roles) and returns the role next sees.
func roleSeen(clientID, topLevel string, appRoles map[string]string) string {
	var got string
	h := AppRole(clientID)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ctxutil.GetUserRole(r.Context())
	}))
	ctx := ctxutil.WithUserRole(httptest.NewRequest(http.MethodGet, "/", nil).Context(), topLevel)
	if appRoles != nil {
		ctx = ctxutil.WithAppRoles(ctx, appRoles)
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	return got
}

func TestAppRoleUsesParashiftRole(t *testing.T) {
	got := roleSeen(parashiftClient, "user", map[string]string{parashiftClient: "admin", "other-app": "user"})
	assert.Equal(t, "admin", got)
}

func TestAppRoleIgnoresTopLevelAdmin(t *testing.T) {
	// A Socrate global admin with no Parashift membership: Socrate puts
	// role=admin in the token, but app_roles has no entry for Parashift.
	got := roleSeen(parashiftClient, "admin", map[string]string{"other-app": "admin"})
	assert.Equal(t, "user", got, "the top-level role must never make a Parashift admin")
}

func TestAppRoleWithoutAppRoles(t *testing.T) {
	assert.Equal(t, "user", roleSeen(parashiftClient, "admin", nil))
}

func TestAppRoleWithoutClientIDKeepsTokenRole(t *testing.T) {
	assert.Equal(t, "manager", roleSeen("", "manager", nil))
}
