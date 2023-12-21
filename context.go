package tui

import (
	"context"
	"strconv"

	"unstable.build/go-tui/term"
)

// ContextWithIteration returns a new Context that holds locker.
func ContextWithIteration(ctx context.Context, i int64) context.Context {
	return term.ContextWithPayload(ctx, []byte(strconv.FormatInt(i, 10)))
}

// IterationFromContext returns the ID value stored in ctx, if any.
func IterationFromContext(ctx context.Context) (int64, bool) {
	payload, ok := term.PayloadFromContext(ctx)
	if !ok {
		return 0, false
	}
	return parsePayload(payload)
}

func parsePayload(payload []byte) (int64, bool) {
	i, err := strconv.ParseInt(string(payload), 10, 64)
	if err != nil {
		return 0, false
	}
	return i, true
}
