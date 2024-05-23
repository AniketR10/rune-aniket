package term

import (
	"context"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type ctxKey int

// pKey is the key for sync.Payload values in Contexts. It is
// unexported; clients use workspace.ContextWithPayload and
// PayloadFromContext instead of using this key directly.
var pKey ctxKey

// ContextWithPayload returns a new Context that holds locker.
func ContextWithPayload(ctx context.Context, payload []byte) context.Context {
	return context.WithValue(ctx, pKey, payload)
}

// PayloadFromContext returns the payload value stored in ctx, if any.
func PayloadFromContext(ctx context.Context) ([]byte, bool) {
	locker, ok := ctx.Value(pKey).([]byte)
	return locker, ok
}
