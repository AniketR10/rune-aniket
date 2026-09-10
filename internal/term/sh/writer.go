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

package sh

import (
	"bytes"
	"context"

	"github.com/unstablebuild/rune-go-sdk/component"
)

// lineWriter is an io.Writer that buffers bytes, splits on
// newlines, and sends each complete line to a channel as a
// component.Responsive. All output uses default attributes.
type lineWriter struct {
	ch  chan<- component.Responsive
	ctx context.Context
	buf bytes.Buffer
}

// Write appends p to the internal buffer, extracts complete
// lines (up to each '\n'), and sends each as a
// ResponsiveString to the channel. It returns the context
// error when the context is done so that callers (e.g.
// io.Copy inside mvdan/sh) stop writing promptly.
func (w *lineWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n := len(p)
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadBytes('\n')
		if err != nil {
			// No more complete lines; put the partial
			// data back into the buffer.
			w.buf.Write(line)
			break
		}
		// Trim the trailing newline.
		text := string(bytes.TrimRight(line, "\n"))
		w.send(text)
	}
	return n, nil
}

// flush sends any remaining partial line in the buffer.
func (w *lineWriter) flush() {
	if w.buf.Len() == 0 {
		return
	}
	w.send(w.buf.String())
	w.buf.Reset()
}

func (w *lineWriter) send(text string) {
	item := component.NewResponsiveString(
		text, component.StringResponsiveConfig{},
	)
	w.push(item)
}

// sendResponsive flushes any buffered partial line and forwards the
// given Responsive as-is to the output channel. Callers use this to
// bypass text flattening when the command output is consumed directly
// by the REPL (rather than piped into another command).
func (w *lineWriter) sendResponsive(item component.Responsive) {
	w.flush()
	w.push(item)
}

func (w *lineWriter) push(item component.Responsive) {
	select {
	case w.ch <- item:
	case <-w.ctx.Done():
	}
}
