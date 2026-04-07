package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHolidaySvc builds a PublicHolidayService wired to the given mock repo.
// The govAPIBase is patched to point at a local test server (if provided).
func newHolidaySvcWithServer(repo *testutil.MockPublicHolidayRepo, apiServer *httptest.Server) *service.PublicHolidayService {
	svc := service.NewPublicHolidayService(repo, newTestLogger())
	if apiServer != nil {
		svc.SetGovAPIBase(apiServer.URL)
	}
	return svc
}

// govAPIServer returns an httptest.Server that serves a French-government-style
// map response: {"YYYY-MM-DD": "Nom du jour"}.
func govAPIServer(t *testing.T, holidays map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(holidays)
	}))
}

// ─── EnsureYear ───────────────────────────────────────────────────────────────

func TestPublicHolidayService_EnsureYear_AlreadyCached(t *testing.T) {
	// If holidays are already in the DB, the API server must NOT be called.
	apiCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	repo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, year int, zone string) ([]*model.PublicHoliday, error) {
			return []*model.PublicHoliday{{
				Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Zone: zone,
				Name: "1er janvier",
			}}, nil
		},
	}

	svc := newHolidaySvcWithServer(repo, srv)
	err := svc.EnsureYear(context.Background(), 2026, "metropole")
	require.NoError(t, err)
	assert.False(t, apiCalled, "API must not be called when data is already in the DB")
}

func TestPublicHolidayService_EnsureYear_FetchesAndStores(t *testing.T) {
	srv := govAPIServer(t, map[string]string{
		"2026-04-06": "Lundi de Pâques",
		"2026-05-01": "Fête du Travail",
	})
	t.Cleanup(srv.Close)

	var upserted []*model.PublicHoliday
	repo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return nil, nil // empty → trigger fetch
		},
		UpsertBatchFn: func(_ context.Context, holidays []*model.PublicHoliday) error {
			upserted = holidays
			return nil
		},
	}

	svc := newHolidaySvcWithServer(repo, srv)
	err := svc.EnsureYear(context.Background(), 2026, "metropole")
	require.NoError(t, err)
	assert.Len(t, upserted, 2)

	dates := make(map[string]string)
	for _, h := range upserted {
		dates[h.Date.Format("2006-01-02")] = h.Name
		assert.Equal(t, "metropole", h.Zone)
	}
	assert.Equal(t, "Lundi de Pâques", dates["2026-04-06"])
	assert.Equal(t, "Fête du Travail", dates["2026-05-01"])
}

func TestPublicHolidayService_EnsureYear_APIDown_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	repo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return nil, nil
		},
	}

	svc := newHolidaySvcWithServer(repo, srv)
	err := svc.EnsureYear(context.Background(), 2026, "metropole")
	require.Error(t, err, "503 from the API should surface as an error")
}

// ─── IsHoliday ────────────────────────────────────────────────────────────────

func TestPublicHolidayService_IsHoliday_True(t *testing.T) {
	easterMonday := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)

	repo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return []*model.PublicHoliday{{Date: easterMonday, Zone: "metropole", Name: "Lundi de Pâques"}}, nil
		},
		GetByDateFn: func(_ context.Context, date time.Time, zone string) (*model.PublicHoliday, error) {
			if date.Day() == 6 && date.Month() == 4 {
				return &model.PublicHoliday{Date: easterMonday, Zone: zone, Name: "Lundi de Pâques"}, nil
			}
			return nil, nil
		},
	}

	svc := newHolidaySvcWithServer(repo, nil)
	ok, name, err := svc.IsHoliday(context.Background(), easterMonday, "metropole")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Lundi de Pâques", name)
}

func TestPublicHolidayService_IsHoliday_False(t *testing.T) {
	regularDay := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC) // Tuesday after Easter

	repo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return []*model.PublicHoliday{{Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Zone: "metropole", Name: "1er janvier"}}, nil
		},
		GetByDateFn: func(_ context.Context, _ time.Time, _ string) (*model.PublicHoliday, error) {
			return nil, nil // not a holiday
		},
	}

	svc := newHolidaySvcWithServer(repo, nil)
	ok, name, err := svc.IsHoliday(context.Background(), regularDay, "metropole")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, name)
}

func TestPublicHolidayService_IsHoliday_APIFailureIsNonFatal(t *testing.T) {
	// When EnsureYear fails (API down + empty DB), IsHoliday returns false rather
	// than blocking scheduling operations.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	repo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, _ int, _ string) ([]*model.PublicHoliday, error) {
			return nil, nil // empty → triggers API call → will fail
		},
	}

	svc := newHolidaySvcWithServer(repo, srv)
	ok, name, err := svc.IsHoliday(context.Background(), time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC), "metropole")
	// Must NOT return an error — scheduling must proceed even when the API is down.
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, name)
}

// ─── ListByYear ───────────────────────────────────────────────────────────────

func TestPublicHolidayService_ListByYear_ReturnsFromDB(t *testing.T) {
	holidays := []*model.PublicHoliday{
		{Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Zone: "metropole", Name: "1er janvier"},
		{Date: time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC), Zone: "metropole", Name: "Lundi de Pâques"},
	}

	repo := &testutil.MockPublicHolidayRepo{
		ListByYearFn: func(_ context.Context, year int, zone string) ([]*model.PublicHoliday, error) {
			assert.Equal(t, 2026, year)
			assert.Equal(t, "metropole", zone)
			return holidays, nil
		},
	}

	svc := newHolidaySvcWithServer(repo, nil)
	got, err := svc.ListByYear(context.Background(), 2026, "metropole")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}
