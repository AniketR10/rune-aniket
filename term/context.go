package term

import (
	"context"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type ctxKey int

// idKey is the key for sync.Payload values in Contexts. It is
// unexported; clients use workspace.ContextWithPayload and
// PayloadFromContext instead of using this key directly.
var lockerKey ctxKey

// ContextWithPayload returns a new Context that holds locker.
func ContextWithPayload(ctx context.Context, payload []byte) context.Context {
	return context.WithValue(ctx, lockerKey, payload)
}

// PayloadFromContext returns the ID value stored in ctx, if any.
func PayloadFromContext(ctx context.Context) ([]byte, bool) {
	locker, ok := ctx.Value(lockerKey).([]byte)
	return locker, ok
}
