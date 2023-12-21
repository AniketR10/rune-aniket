package browser

import (
	"context"

	"unstable.build/go-tui/term"
)

// EventPublisherInterrupter wraps an EventPublisher to satisfy term.Interrupter.
func EventPublisherInterrupter(e EventPublisher) term.Interrupter {
	return pubInterrupt{e: e}
}

type pubInterrupt struct {
	e EventPublisher
}

func (p pubInterrupt) Interrupt(ctx context.Context) error {
	payload, _ := term.PayloadFromContext(ctx)
	return p.e.PublishEvent(term.Event{Type: term.EventInterrupt, Raw: payload})
}
