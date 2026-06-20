// Package httpx provides resilient helpers for outbound HTTP calls: bounded
// response-body reads and retry-with-backoff on transient failures (OBS-4).
package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultMaxBodyBytes caps how much of an external response we read, to defend
// against oversized / decompression-bomb responses.
const DefaultMaxBodyBytes int64 = 5 << 20 // 5 MiB

// Options tunes GetWithRetry. Zero values fall back to sensible defaults.
type Options struct {
	MaxAttempts  int           // total attempts including the first (default 3)
	BaseBackoff  time.Duration // backoff before the first retry, doubled each time (default 200ms)
	MaxBodyBytes int64         // response body cap (default DefaultMaxBodyBytes)
}

func (o Options) withDefaults() Options {
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 3
	}
	if o.BaseBackoff <= 0 {
		o.BaseBackoff = 200 * time.Millisecond
	}
	if o.MaxBodyBytes <= 0 {
		o.MaxBodyBytes = DefaultMaxBodyBytes
	}
	return o
}

// transient reports whether the outcome is worth retrying: a transport error or
// a 5xx/429 response.
func transient(statusCode int, err error) bool {
	if err != nil {
		return true
	}
	return statusCode >= 500 || statusCode == http.StatusTooManyRequests
}

// GetWithRetry issues GET url and returns the (size-capped) response body for a
// 2xx response. Transient failures (transport errors, 5xx, 429) are retried with
// exponential backoff up to opt.MaxAttempts; non-2xx-non-transient responses
// fail immediately. After exhausting attempts the last error is returned so the
// caller can degrade gracefully. Honors context cancellation between attempts.
func GetWithRetry(ctx context.Context, client *http.Client, url string, headers map[string]string, opt Options) ([]byte, error) {
	opt = opt.withDefaults()
	var lastErr error
	backoff := opt.BaseBackoff

	for attempt := 1; attempt <= opt.MaxAttempts; attempt++ {
		body, status, err := doGet(ctx, client, url, headers, opt.MaxBodyBytes)
		if err == nil && status >= 200 && status < 300 {
			return body, nil
		}

		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("unexpected HTTP status %d", status)
		}

		// Don't retry client errors (4xx other than 429): they won't change.
		if !transient(status, err) {
			return nil, lastErr
		}
		if attempt == opt.MaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return nil, fmt.Errorf("request to %s failed after %d attempts: %w", url, opt.MaxAttempts, lastErr)
}

func doGet(ctx context.Context, client *http.Client, url string, headers map[string]string, maxBytes int64) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	// Bound the body read regardless of status.
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if readErr != nil {
		return nil, resp.StatusCode, readErr
	}
	return body, resp.StatusCode, nil
}
