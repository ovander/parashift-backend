package httpx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovander/parashift/internal/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fastOpts() httpx.Options {
	return httpx.Options{MaxAttempts: 3, BaseBackoff: time.Millisecond, MaxBodyBytes: httpx.DefaultMaxBodyBytes}
}

// OBS-4: transient 5xx responses are retried, then the call succeeds.
func TestGetWithRetry_RetriesThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	body, err := httpx.GetWithRetry(context.Background(), srv.Client(), srv.URL, nil, fastOpts())
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(body))
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "should have retried twice before success")
}

// OBS-4: after exhausting attempts on persistent 5xx, it fails gracefully.
func TestGetWithRetry_GivesUpAfterMax(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	opt := fastOpts()
	opt.MaxAttempts = 2
	_, err := httpx.GetWithRetry(context.Background(), srv.Client(), srv.URL, nil, opt)
	require.Error(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "must stop after MaxAttempts")
}

// OBS-4: client errors (4xx, not 429) are not retried — they won't change.
func TestGetWithRetry_NoRetryOn4xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := httpx.GetWithRetry(context.Background(), srv.Client(), srv.URL, nil, fastOpts())
	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "4xx must not be retried")
}

// OBS-4: the response body is bounded by MaxBodyBytes.
func TestGetWithRetry_CapsBody(t *testing.T) {
	big := strings.Repeat("A", 1<<20) // 1 MiB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(big))
	}))
	defer srv.Close()

	opt := fastOpts()
	opt.MaxBodyBytes = 100
	body, err := httpx.GetWithRetry(context.Background(), srv.Client(), srv.URL, nil, opt)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(body), 100, "body must be capped by MaxBodyBytes")
}

// Context cancellation aborts the retry loop promptly.
func TestGetWithRetry_RespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opt := fastOpts()
	opt.BaseBackoff = time.Second // would block if cancellation weren't honored
	_, err := httpx.GetWithRetry(ctx, srv.Client(), srv.URL, nil, opt)
	require.Error(t, err)
}
