package parser

import "time"

// Timeout provides an interface for creating timeouts and checking their expiry.
type Timeout interface {
	// SetTimeout sets the timeout for the next synchronized update.
	//
	// The duration parameter specifies the duration of the timeout. Once the
	// specified duration has elapsed, the synchronized update routine can be
	// performed.
	SetTimeout(duration time.Duration)

	// ClearTimeout clears the current timeout.
	ClearTimeout()

	// PendingTimeout returns whether a timeout is currently active and has not yet expired.
	PendingTimeout() bool
}

// StdTimeout represents a std library's Timeout implementation.
// A zero value is ready to be used.
type StdTimeout struct {
	timeout time.Time
}

// SyncTimeout returns the synchronized update expiration time.
func (h *StdTimeout) SyncTimeout() time.Time {
	return h.timeout
}

// SetTimeout sets the timeout for the next synchronized update.
func (h *StdTimeout) SetTimeout(duration time.Duration) {
	h.timeout = time.Now().Add(duration)
}

// ClearTimeout clears the current timeout.
func (h *StdTimeout) ClearTimeout() {
	h.timeout = time.Time{}
}

// PendingTimeout returns whether a timeout is currently active and has not yet expired.
func (h *StdTimeout) PendingTimeout() bool {
	return time.Now().Before(h.timeout)
}
