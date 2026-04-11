package pkg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithLocale_GetLocale(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty string when not set", func(t *testing.T) {
		assert.Equal(t, "", GetLocale(ctx))
	})

	t.Run("returns fr when set to fr", func(t *testing.T) {
		ctx2 := WithLocale(ctx, "fr")
		assert.Equal(t, "fr", GetLocale(ctx2))
	})

	t.Run("returns en when set to en", func(t *testing.T) {
		ctx2 := WithLocale(ctx, "en")
		assert.Equal(t, "en", GetLocale(ctx2))
	})

	t.Run("inner context overrides outer context", func(t *testing.T) {
		outer := WithLocale(ctx, "fr")
		inner := WithLocale(outer, "en")
		assert.Equal(t, "en", GetLocale(inner))
		// Outer is unchanged
		assert.Equal(t, "fr", GetLocale(outer))
	})
}
