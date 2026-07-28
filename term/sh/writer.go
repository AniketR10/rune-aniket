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
