package plugin

import (
	"context"
	"sync"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type ctxKey int

// wgKey is the key for sync.WaitGroup values in Contexts
var wgKey ctxKey

// ContextWithWaitGroup returns a new Context that stores wg.
func ContextWithWaitGroup(ctx context.Context, wg *sync.WaitGroup) context.Context {
	return context.WithValue(ctx, wgKey, wg)
}

// WaitGroupFromContext returns this context's wait group or panics
// if this context does not have a waitgroup.
func WaitGroupFromContext(ctx context.Context) *sync.WaitGroup {
	wg := ctx.Value(wgKey).(*sync.WaitGroup)
	if wg == nil {
		panic("WaitGroupContext called on an invalid context")
	}
	return wg
}
