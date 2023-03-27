package workspace

import (
	"context"
	"sync"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type ctxKey int

// idKey is the key for sync.Locker values in Contexts. It is
// unexported; clients use workspace.ContextWithLocker and
// LockerFromContext instead of using this key directly.
var lockerKey ctxKey

// ContextWithLocker returns a new Context that holds locker.
func ContextWithLocker(ctx context.Context, locker sync.Locker) context.Context {
	return context.WithValue(ctx, lockerKey, locker)
}

// LockerFromContext returns the ID value stored in ctx, if any.
func LockerFromContext(ctx context.Context) (sync.Locker, bool) {
	locker, ok := ctx.Value(lockerKey).(sync.Locker)
	return locker, ok
}
