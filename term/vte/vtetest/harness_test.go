// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package vtetest

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/debug"
)

// slowEchoHandler models a shell whose echo lands later than the
// harness' quiescence timeout, which is what a saturated host does to
// the vte integration tests.
type slowEchoHandler struct {
	delay     time.Duration
	interrupt chan struct{}

	mu    sync.Mutex
	typed []rune
}

func (h *slowEchoHandler) Handle(ev term.Event) (exit, handled bool) {
	go debug.CapturePanicReport(func() {
		time.Sleep(h.delay)
		h.mu.Lock()
		h.typed = append(h.typed, ev.Ch)
		h.mu.Unlock()
		h.interrupt <- struct{}{}
	})
	return false, true
}

func (h *slowEchoHandler) Draw(w term.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for x, r := range h.typed {
		w.SetCell(term.Coordinates{X: x}, term.Cell{Ch: r, Width: 1, Bytes: 1})
	}
}

func (h *slowEchoHandler) Resize(int, int)           {}
func (h *slowEchoHandler) Selection() (string, bool) { return "", false }
func (h *slowEchoHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

// TestHandleTestCaseConvergesOnLateEcho pins the settle contract: a
// response arriving after the quiescence timeout must still be
// observed, otherwise every vte integration test is one scheduling
// hiccup away from asserting on a half-drawn screen.
func TestHandleTestCaseConvergesOnLateEcho(t *testing.T) {
	const (
		drawTimeout   = 10 * time.Millisecond
		width, height = 4, 1
	)
	h := &slowEchoHandler{
		delay:     8 * drawTimeout,
		interrupt: make(chan struct{}, 8),
	}
	cases := []Case{{
		InputSequence: "ab",
		Expected:      "ab" + strings.Repeat(" ", width-2),
	}}

	settled := t.Run("settle", func(t *testing.T) {
		TestCases(t, h, width, height, drawTimeout, h.interrupt, cases)
	})
	assert.True(t, settled,
		"harness must converge on an echo slower than drawTimeout")
}
