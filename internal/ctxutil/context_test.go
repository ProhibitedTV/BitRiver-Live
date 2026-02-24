package ctxutil

import (
	"context"
	"testing"
)

func TestNormalize(t *testing.T) {
	t.Run("nil context returns background", func(t *testing.T) {
		if got := Normalize(nil); got != context.Background() {
			t.Fatalf("expected context.Background for nil context")
		}
	})

	t.Run("non nil context passes through", func(t *testing.T) {
		type testKey string
		ctx := context.WithValue(context.Background(), testKey("key"), "value")
		if got := Normalize(ctx); got != ctx {
			t.Fatalf("expected original context to be returned")
		}
	})
}
