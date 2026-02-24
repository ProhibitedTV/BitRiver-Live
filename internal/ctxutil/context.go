package ctxutil

import "context"

// Normalize returns context.Background when ctx is nil.
func Normalize(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
