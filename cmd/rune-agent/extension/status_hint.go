// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package extension

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/debug"
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
