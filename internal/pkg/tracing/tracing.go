// Package tracing wires OpenTelemetry distributed tracing (OBS-3). It is
// deployment-agnostic and disabled by default: with no OTLP endpoint configured
// Init installs nothing and returns a no-op shutdown, so the binary runs
// identically until an operator points it at a collector.
package tracing

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ovander/backendkit/ctxutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Config controls tracer setup. Endpoint is the OTLP/HTTP collector address
// (host:port, no scheme); an empty Endpoint disables tracing entirely.
type Config struct {
	Endpoint       string
	Insecure       bool
	SampleRatio    float64
	ServiceName    string
	ServiceVersion string
	Environment    string
}

// scopeName identifies this instrumentation in emitted spans.
const scopeName = "github.com/ovander/parashift/internal/pkg/tracing"

// Init configures the global OpenTelemetry tracer provider and propagator from
// cfg and returns a shutdown function that flushes pending spans. When
// cfg.Endpoint is empty it is a no-op (tracing disabled) and the returned
// shutdown does nothing.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if cfg.Endpoint == "" {
		return noop, nil
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return noop, err
	}

	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironment(cfg.Environment),
	))
	if err != nil {
		res = resource.Default()
	}

	ratio := cfg.SampleRatio
	if ratio <= 0 {
		ratio = 1.0
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		// ParentBased respects an upstream sampling decision; otherwise sample at ratio.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return tp.Shutdown, nil
}

// statusRecorder captures the response status code for span/metric attributes.
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
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(b)
}

// Middleware starts a server span for each request, continuing any trace
// propagated by the caller. The span is named by the chi route pattern (filled
// after routing) to keep span names low-cardinality. The active trace_id is
// echoed in the X-Trace-Id response header and added to the request-scoped
// logger so application logs correlate with traces.
//
// It is always safe to install: when tracing is disabled the global provider is
// a no-op, spans are not recorded and there is no exporter, so the only cost is
// a cheap no-op span and the trace_id plumbing.
func Middleware(next http.Handler) http.Handler {
	tracer := otel.Tracer(scopeName)
	propagator := otel.GetTextMapPropagator()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, r.Method,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.URLPath(r.URL.Path),
			),
		)
		defer span.End()

		// Correlate logs with the trace.
		if sc := span.SpanContext(); sc.HasTraceID() {
			traceID := sc.TraceID().String()
			w.Header().Set("X-Trace-Id", traceID)
			ctx = ctxutil.WithLogger(ctx, ctxutil.GetLogger(ctx).WithField("trace_id", traceID))
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r.WithContext(ctx))

		// Route pattern is known only after chi has matched the route.
		route := routePattern(r)
		span.SetName(r.Method + " " + route)
		span.SetAttributes(
			attribute.String("http.route", route),
			semconv.HTTPResponseStatusCode(rec.status),
			attribute.Int64("http.server.duration_ms", time.Since(start).Milliseconds()),
		)
		if rec.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(rec.status))
		}
	})
}

// routePattern returns the chi route template, or "unmatched" for unknown paths,
// keeping span-name cardinality bounded by the number of registered routes.
func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}
