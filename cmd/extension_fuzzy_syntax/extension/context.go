package extension

import (
	"context"
)

type ctxKey int

var queryKey ctxKey

func contextWithQuery(ctx context.Context, query string) context.Context {
	return context.WithValue(ctx, queryKey, query)
}

func queryFromContext(ctx context.Context) (string, bool) {
	query, ok := ctx.Value(queryKey).(string)
	return query, ok
}
