package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestE2E_Health verifies that GET /healthz is publicly accessible and returns 200.
func TestE2E_Health(t *testing.T) {
	ts := newTestServer(t, emptyMocks())

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("get /healthz: %v", err)
	}
	drain(resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
