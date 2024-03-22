package screen

import (
	"context"
)

type ctxKey int

var screenIDKey ctxKey

func screenContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, screenIDKey, struct{}{})
}

// IsScreenContext returns the ID value stored in ctx, if any.
func IsScreenContext(ctx context.Context) bool {
	v := ctx.Value(screenIDKey)
	return v != nil
}
