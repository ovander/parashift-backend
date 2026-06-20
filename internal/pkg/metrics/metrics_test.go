package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ovander/parashift/internal/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRouter builds a chi router with the metrics middleware + /metrics endpoint
// installed, plus a couple of representative routes.
func newRouter() (*chi.Mux, *metrics.Collector) {
	mc := metrics.New()
	r := chi.NewRouter()
	r.Use(mc.Middleware)
	r.Method(http.MethodGet, "/metrics", mc.Handler())
	r.Get("/stores/{storeId}/schedule", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/boom", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	return r, mc
}

func scrape(t *testing.T, r http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

// OBS-2: /metrics exposes the exposition format and the standard RED series.
// (CounterVec/HistogramVec series only materialise after the first observation,
// so we drive one request through a counted route first.)
func TestMetrics_EndpointExposesREDSeries(t *testing.T) {
	r, _ := newRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stores/1/schedule", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body := scrape(t, r)
	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "http_request_duration_seconds")
	assert.Contains(t, body, "http_requests_in_flight")
}

// OBS-2: requests are counted, labelled by the chi route *pattern* (not the
// concrete path, so path params don't explode cardinality) and by status.
func TestMetrics_CountsByRoutePatternAndStatus(t *testing.T) {
	r, _ := newRouter()

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stores/42/schedule", nil))
		require.Equal(t, http.StatusOK, rec.Code)
	}

	body := scrape(t, r)
	// The label uses the template, not "/stores/42/schedule".
	assert.Contains(t, body,
		`http_requests_total{method="GET",route="/stores/{storeId}/schedule",status="200"} 3`)
	assert.NotContains(t, body, "/stores/42/schedule")
}

// OBS-2: error responses are captured with their status code.
func TestMetrics_CapturesErrorStatus(t *testing.T) {
	r, _ := newRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	body := scrape(t, r)
	assert.Contains(t, body,
		`http_requests_total{method="GET",route="/boom",status="500"} 1`)
}

// OBS-2: unmatched paths collapse to a single "unmatched" label rather than
// creating one series per bad URL.
func TestMetrics_UnmatchedPathsBounded(t *testing.T) {
	r, _ := newRouter()

	for _, p := range []string{"/nope/a", "/nope/b", "/nope/c"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		require.Equal(t, http.StatusNotFound, rec.Code)
	}

	body := scrape(t, r)
	assert.Contains(t, body, `route="unmatched"`)
	for _, p := range []string{"/nope/a", "/nope/b", "/nope/c"} {
		assert.NotContains(t, body, p)
	}
}

// OBS-2: duration histogram records observations (count > 0) per route.
func TestMetrics_DurationHistogramObserved(t *testing.T) {
	r, _ := newRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stores/7/schedule", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body := scrape(t, r)
	var found bool
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "http_request_duration_seconds_count") &&
			strings.Contains(line, `route="/stores/{storeId}/schedule"`) {
			found = true
			assert.True(t, strings.HasSuffix(strings.TrimSpace(line), "1"),
				"expected one observation, got: %s", line)
		}
	}
	assert.True(t, found, "duration_count series for the route should be present")
}
