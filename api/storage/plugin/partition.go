package plugin

import (
	"context"

	"unstable.build/go-tui/plugin"
)

// WithPartition creates a storage partition with name, such that
// calls to plugin.Storage will return a document.Service that's
// logically partition from the rest.
func WithPartition(grant plugin.Grant, name string) plugin.Grant {
	grant.Context = contextWithPartition(grant.Context, name)
	return grant
}

type ctxKey int

var partitionKey ctxKey

func contextWithPartition(ctx context.Context, partition string) context.Context {
	return context.WithValue(ctx, partitionKey, partition)
}

func partitionFromContext(ctx context.Context) (string, bool) {
	partition, ok := ctx.Value(partitionKey).(string)
	return partition, ok
}
