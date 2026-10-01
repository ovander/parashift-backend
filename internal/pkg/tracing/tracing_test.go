package tracing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ovander/parashift/internal/pkg/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// installRecorder wires a recording tracer provider + propagator as the globals
// and returns the recorder plus a cleanup that restores no-op globals.
func installRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	})
	return rec
}

func routerWith(h http.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(tracing.Middleware)
	r.Get("/stores/{storeId}/schedule", h.ServeHTTP)
	return r
}

// OBS-3: with no endpoint configured, Init disables tracing and returns a
// working no-op shutdown.
func TestInit_DisabledIsNoop(t *testing.T) {
	shutdown, err := tracing.Init(context.Background(), tracing.Config{})
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}

// OBS-3: a request produces a server span named by the chi route pattern, with
// route + status attributes, and echoes the trace id in X-Trace-Id.
func TestMiddleware_RecordsServerSpan(t *testing.T) {
	rec := installRecorder(t)
	r := routerWith(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/stores/7/schedule", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("X-Trace-Id"), "trace id should be echoed for correlation")

	spans := rec.Ended()
	require.Len(t, spans, 1)
	span := spans[0]
	assert.Equal(t, "GET /stores/{storeId}/schedule", span.Name(),
		"span should be named by the low-cardinality route pattern")
	assert.Equal(t, oteltrace.SpanKindServer, span.SpanKind())

	attrs := map[string]string{}
	for _, kv := range span.Attributes() {
		attrs[string(kv.Key)] = kv.Value.String()
	}
	assert.Equal(t, "/stores/{storeId}/schedule", attrs["http.route"])
}

// OBS-3: 5xx responses mark the span as errored.
func TestMiddleware_MarksServerErrors(t *testing.T) {
	rec := installRecorder(t)
	r := routerWith(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/stores/7/schedule", nil))

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "Error", spans[0].Status().Code.String())
}

// OBS-3: an incoming traceparent is continued (same trace id, server span is a child).
func TestMiddleware_PropagatesTraceContext(t *testing.T) {
	rec := installRecorder(t)
	r := routerWith(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/stores/7/schedule", nil)
	// Valid W3C traceparent with a known trace id.
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	req.Header.Set("traceparent", "00-"+traceID+"-00f067aa0ba902b7-01")

	r.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, traceID, spans[0].SpanContext().TraceID().String(),
		"server span should continue the propagated trace")
	assert.True(t, spans[0].Parent().IsValid(), "server span should have the remote parent")
}

// OBS-3: when tracing is disabled (no-op global provider) the middleware still
// serves the request and records nothing.
func TestMiddleware_DisabledStillServes(t *testing.T) {
	otel.SetTracerProvider(tracenoop.NewTracerProvider())
	r := routerWith(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/stores/7/schedule", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
}
