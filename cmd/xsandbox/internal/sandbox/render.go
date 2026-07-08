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

package sandbox

import (
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// defaultRenderWidth and defaultRenderHeight size the headless browser
// component at startup. The size is fixed so window installs resize
// their new child synchronously, flushing the install-response
// handshake back to the extension; render() can override it per call.
const (
	defaultRenderWidth  = 80
	defaultRenderHeight = 24
)

// render resizes the headless browser component to width x height and
// draws it into an in-memory cell grid, returning the grid as a plain
// string (one line per row, trailing blanks trimmed). It composites
// every installed handler exactly as a real terminal would.
func (h *browserHost) render(width, height int) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.width, h.height = width, height
	w := term.NewStringWriter(width, height)
	h.comp.Resize(width, height)
	h.comp.Draw(w)
	if cur, _, show := h.comp.Cursor(); show {
		w.SetCursor(cur)
	}
	_ = w.Flush()
	return trimGrid(w.String())
}

// sendKeys parses a handlertest-style key sequence and delivers each
// resulting key event to the component's focused handler, returning
// whether the last event was handled and whether it requested exit.
func (h *browserHost) sendKeys(sequence string) (handled, quit bool, err error) {
	keys, err := term.ParseKeys(sequence)
	if err != nil {
		return false, false, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, key := range keys {
		ev := term.Event{Ch: key.Ch, Mod: key.Mod, Key: key.Key, Type: term.EventKey}
		quit, handled = h.comp.Handle(ev)
	}
	return handled, quit, nil
}

// trimGrid removes trailing whitespace from each row and trailing
// blank rows so rendered output is easy to assert against.
func trimGrid(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
