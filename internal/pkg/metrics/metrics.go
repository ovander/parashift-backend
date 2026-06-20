// Package metrics provides Prometheus RED (Rate, Errors, Duration) metrics for
// every HTTP route, plus an exposition handler for scraping (OBS-2).
//
// Metrics are labelled by the chi *route pattern* (e.g. "/stores/{storeId}/schedule")
// rather than the concrete path, so that path parameters do not explode label
// cardinality.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Collector owns the metric vectors and a private Prometheus registry.
// A private registry (rather than the global default) keeps tests isolated and
// avoids accidental duplicate-registration panics.
type Collector struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

// New builds a Collector with the standard process/Go runtime collectors plus
// the HTTP RED metrics.
func New() *Collector {
	reg := prometheus.NewRegistry()

	c := &Collector{
		registry: reg,
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests, by method, route and status code.",
			},
			[]string{"method", "route", "status"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds, by method and route.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route"},
		),
		inFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being served.",
			},
		),
	}

	reg.MustRegister(
		c.requests,
		c.duration,
		c.inFlight,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return c
}

// Handler returns the Prometheus exposition handler for GET /metrics.
func (c *Collector) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

// statusRecorder captures the response status code so it can be used as a label.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		// Implicit 200 when a handler writes without calling WriteHeader.
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(b)
}

// Middleware records RED metrics for each request. It must be installed after
// the chi router has matched a route so that the route pattern is available;
// for unmatched paths the pattern falls back to "unmatched" to bound cardinality.
func (c *Collector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		c.inFlight.Inc()
		defer c.inFlight.Dec()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		route := routePattern(r)
		c.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		c.duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// routePattern returns the chi route template for the request, or "unmatched"
// when no route matched (404s on unknown paths). Using the template keeps the
// "route" label cardinality bounded by the number of registered routes.
func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}
