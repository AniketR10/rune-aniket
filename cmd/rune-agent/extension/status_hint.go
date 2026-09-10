// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package extension

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/debug"
)

// statusPhase describes the current phase of the agent turn.
type statusPhase int

const (
	phaseSending     statusPhase = iota // sending request to LLM
	phaseThinking                       // waiting for first content token
	phaseReceiving                      // receiving streamed content
	phaseToolCalling                    // tools are executing
	phaseCompacting                     // conversation is being compacted
	phaseRateLimited                    // waiting due to rate limit / retry
)

func (p statusPhase) String() string {
	switch p {
	case phaseSending:
		return "sending"
	case phaseThinking:
		return "thinking"
	case phaseReceiving:
		return "receiving"
	case phaseToolCalling:
		return "tool calling"
	case phaseCompacting:
		return "compacting"
	case phaseRateLimited:
		return "rate limited"
	default:
		return ""
	}
}

var statusSpinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// statusHint is a tui.Component that displays a dynamic status line
// with a spinner, phase label, elapsed time, and token count.
type statusHint struct {
	mu                sync.Mutex
	interrupter       term.Interrupter
	phase             statusPhase
	startTime         time.Time
	durationPrecision time.Duration // when positive, truncates elapsed to this granularity
	tokensSent        int           // cumulative input tokens
	tokensReceived    int           // cumulative output tokens
	width             int
	drawCount         int
	attr              term.Attributes // background attr; text uses Fg from it
	cancel            context.CancelFunc
	activeFormFn      func() string // optional; returns activeForm from progress widget
}

func newStatusHint(interrupter term.Interrupter, attr term.Attributes, durationPrecision time.Duration, activeFormFn func() string) *statusHint {
	ctx, cancel := context.WithCancel(context.Background())
	h := &statusHint{
		interrupter:       interrupter,
		startTime:         time.Now(),
		durationPrecision: durationPrecision,
		attr:              attr,
		cancel:            cancel,
		activeFormFn:      activeFormFn,
	}
	go debug.CapturePanicReport(func() {
		h.tick(ctx)
	})
	return h
}

func (h *statusHint) tick(ctx context.Context) {
	ticker := time.NewTicker(125 * time.Millisecond) // 8 Hz
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = h.interrupter.Interrupt(ctx)
		}
	}
}

func (h *statusHint) Close() error {
	h.cancel()
	return nil
}

func (h *statusHint) setPhase(p statusPhase) {
	h.mu.Lock()
	h.phase = p
	h.mu.Unlock()
}

func (h *statusHint) setTokens(sent, received int) {
	h.mu.Lock()
	h.tokensSent = sent
	h.tokensReceived = received
	h.mu.Unlock()
}

func (h *statusHint) Resize(width, height int) {
	h.mu.Lock()
	h.width = width
	h.mu.Unlock()
}

func (h *statusHint) Draw(w term.Writer) {
	h.mu.Lock()
	phase := h.phase
	tokensSent := h.tokensSent
	tokensReceived := h.tokensReceived
	width := h.width
	h.drawCount++
	dc := h.drawCount
	h.mu.Unlock()

	spinner := statusSpinnerFrames[dc%len(statusSpinnerFrames)]
	elapsed := time.Since(h.startTime)
	if h.durationPrecision > 0 {
		elapsed = elapsed.Truncate(h.durationPrecision)
	}

	phaseLabel := phase.String()
	if h.activeFormFn != nil {
		if af := h.activeFormFn(); af != "" {
			phaseLabel = af
		}
	}

	var sb strings.Builder
	sb.WriteRune(spinner)
	sb.WriteByte(' ')
	sb.WriteString(phaseLabel)
	sb.WriteString(" (")
	sb.WriteString(formatStatusDuration(elapsed))
	if tokensSent > 0 || tokensReceived > 0 {
		sb.WriteString(" · ↑ ")
		sb.WriteString(formatTokenCount(tokensSent))
		sb.WriteString(" · ↓ ")
		sb.WriteString(formatTokenCount(tokensReceived))
	}
	sb.WriteByte(')')

	cellAttr := term.Attributes{Bg: h.attr.Bg}
	text := sb.String()
	x := 0
	for _, r := range text {
		if x >= width {
			break
		}
		w.SetCell(term.Coordinates{X: x, Y: 0}, term.NewCell(r, 1, cellAttr))
		x++
	}
}

// formatStatusDuration formats a duration for the status line.
func formatStatusDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s %= 60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m %= 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// formatTokenCount formats a token count for the status line.
func formatTokenCount(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d tokens", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fk tokens", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1fm tokens", float64(n)/1_000_000)
	}
}

// formatTokenNumber formats a token count without the "tokens" suffix and
// without decimal places.
func formatTokenNumber(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.0fm", float64(n)/1_000_000)
	}
}
