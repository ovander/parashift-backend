package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── GET /public-holidays ─────────────────────────────────────────────────────

func TestE2E_PublicHolidays_List(t *testing.T) {
	jan1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	easterMonday := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)

	mgrSub := "sub-mgr-holiday-list"
	mocks := emptyMocks()
	storeID := uuid.New()
	withManagerEmp(mocks, storeID, mgrSub)
	mocks.publicHoliday = &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, year int, zone string) ([]*model.PublicHoliday, error) {
			if year == 2026 && zone == "metropole" {
				return []*model.PublicHoliday{
					{Date: jan1, Zone: "metropole", Name: "1er janvier"},
					{Date: easterMonday, Zone: "metropole", Name: "Lundi de Pâques"},
				}, nil
			}
			return nil, nil
		},
	}

	ts := newTestServer(t, mocks)
	c := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := c.do(t, http.MethodGet, "/public-holidays?year=2026", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body []map[string]any
	decode(t, resp, &body)

	require.Len(t, body, 2)
	dates := make(map[string]string)
	for _, h := range body {
		dates[h["date"].(string)] = h["name"].(string)
	}
	assert.Equal(t, "1er janvier", dates["2026-01-01"])
	assert.Equal(t, "Lundi de Pâques", dates["2026-04-06"])
}

func TestE2E_PublicHolidays_DefaultsToCurrentYear(t *testing.T) {
	mgrSub := "sub-mgr-holiday-year"
	mocks := emptyMocks()
	storeID := uuid.New()
	withManagerEmp(mocks, storeID, mgrSub)

	queriedYear := 0
	mocks.publicHoliday = &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, year int, _ string) ([]*model.PublicHoliday, error) {
			queriedYear = year
			return nil, nil
		},
	}

	ts := newTestServer(t, mocks)
	c := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := c.do(t, http.MethodGet, "/public-holidays", nil)
	drain(resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, time.Now().Year(), queriedYear)
}

func TestE2E_PublicHolidays_InvalidYear(t *testing.T) {
	mgrSub := "sub-mgr-holiday-invalid"
	mocks := emptyMocks()
	storeID := uuid.New()
	withManagerEmp(mocks, storeID, mgrSub)

	ts := newTestServer(t, mocks)
	c := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := c.do(t, http.MethodGet, "/public-holidays?year=banana", nil)
	drain(resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestE2E_PublicHolidays_ZoneParam(t *testing.T) {
	mgrSub := "sub-mgr-holiday-zone"
	mocks := emptyMocks()
	storeID := uuid.New()
	withManagerEmp(mocks, storeID, mgrSub)

	queriedZone := ""
	mocks.publicHoliday = &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, zone string) ([]*model.PublicHoliday, error) {
			queriedZone = zone
			return nil, nil
		},
	}

	ts := newTestServer(t, mocks)
	c := newClient(ts, storeID, uuid.New(), mgrSub, "manager")

	resp := c.do(t, http.MethodGet, "/public-holidays?year=2026&zone=alsace-moselle", nil)
	drain(resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "alsace-moselle", queriedZone)
}

func TestE2E_PublicHolidays_RequiresAuth(t *testing.T) {
	ts := newTestServer(t, emptyMocks())

	// Raw GET with no auth headers — testAuthMiddleware won't inject a tenant/user,
	// so TenantMiddleware should reject with 401.
	resp, err := http.Get(ts.URL + "/api/v1/public-holidays?year=2026")
	require.NoError(t, err)
	drain(resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
