package pkg

import "context"

// localeContextKey is the unexported type for the locale context key.
type localeContextKey struct{}

// WithLocale returns a new context with the request locale set.
// Valid values are "fr" (French) and "en" (English).
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey{}, locale)
}

// GetLocale extracts the request locale from context.
// Returns "" when absent; callers should default to "fr" for the Parashift app.
func GetLocale(ctx context.Context) string {
	if v, ok := ctx.Value(localeContextKey{}).(string); ok {
		return v
	}
	return ""
}
