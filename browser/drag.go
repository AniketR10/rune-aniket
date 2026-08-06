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

package browser

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/debug"
)

var _ DragTarget = (*Component)(nil)

const (
	dragVeilFPS         = 24
	dragVeilPeriodFrame = 36
	dragVeilIntensity   = 0.45
	defaultDropLabel    = "Drop files here"
)

// dragState is the drop-target overlay state. Every field except frame
// is owned by the event loop; frame is advanced by the veil ticker.
type dragState struct {
	win    *browserWindow
	cancel func()
	buf    *cell.BufferWriter
	bufW   int
	bufH   int
	frame  atomic.Int64
}

// SetInterrupter installs the interrupter used to animate the
// drop-target veil. Without one the veil is still drawn, but only
// repaints when the host redraws for another reason.
func (c *Component) SetInterrupter(i term.Interrupter) {
	c.interrupter = i
}

// WindowAt returns the window rendered at pos, which is relative to
// this Component's top-left corner.
func (c *Component) WindowAt(pos term.Coordinates) (Window, bool) {
	off := c.WindowManagerPosition()
	pos.X -= off.X
	pos.Y -= off.Y
	if pos.X < 0 || pos.Y < 0 {
		return nil, false
	}
	win, ok := c.wm.WindowAt(pos)
	if !ok {
		return nil, false
	}
	bwin, ok := c.findWindow(win.ID())
	if !ok {
		return nil, false
	}
	return bwin, true
}

// DragHover marks the window under pos as the pending drop target and
// starts the veil animation. It reports whether a window was found;
// when none is, any previous target is cleared.
func (c *Component) DragHover(pos term.Coordinates) bool {
	win, ok := c.WindowAt(pos)
	if !ok {
		c.DragCancel()
		return false
	}
	bwin := win.(*browserWindow)
	if c.drag.win == bwin {
		return true
	}
	c.drag.win = bwin
	if c.drag.cancel == nil {
		c.startDragVeil()
	}
	c.interrupt()
	return true
}

// DragCancel clears the drop target and stops the veil animation.
func (c *Component) DragCancel() {
	if c.drag.win == nil && c.drag.cancel == nil {
		return
	}
	c.drag.win = nil
	if c.drag.cancel != nil {
		c.drag.cancel()
		c.drag.cancel = nil
	}
	c.drag.frame.Store(0)
	c.interrupt()
}

// DragDrop delivers paths to the window under pos as a bracketed paste,
// focusing that window first so the paste reaches it even when another
// window holds the focus. It reports whether a window received the drop.
func (c *Component) DragDrop(pos term.Coordinates, paths []string) bool {
	c.DragCancel()
	if len(paths) == 0 {
		return false
	}
	win, ok := c.WindowAt(pos)
	if !ok {
		return false
	}
	c.SetFocus(win)
	c.Handle(term.Event{Type: term.EventPasteStart})
	for _, r := range strings.Join(paths, "\n") {
		c.Handle(term.Event{
			Type: term.EventKey,
			Ch:   r,
			Raw:  []byte(string(r)),
		})
	}
	c.Handle(term.Event{Type: term.EventPasteEnd})
	return true
}

func (c *Component) startDragVeil() {
	if c.interrupter == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.drag.cancel = cancel
	interrupter := c.interrupter
	frame := &c.drag.frame
	go debug.CapturePanicReport(func() {
		ticker := time.NewTicker(time.Second / dragVeilFPS)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				frame.Add(1)
				_ = interrupter.Interrupt(ctx)
			case <-ctx.Done():
				return
			}
		}
	})
}

func (c *Component) interrupt() {
	if c.interrupter == nil {
		return
	}
	_ = c.interrupter.Interrupt(context.Background())
}

// drawDragVeil renders the browser with a pulsing veil over the drop
// target. It reports false when no drop target is active, leaving the
// caller to draw normally.
func (c *Component) drawDragVeil(w term.Writer) bool {
	win := c.drag.win
	if win == nil || win.Closed() {
		return false
	}
	width, height := win.Width(), win.Height()
	if width <= 0 || height <= 0 || c.width <= 0 || c.height <= 0 {
		return false
	}
	if c.drag.buf == nil || c.drag.bufW != c.width || c.drag.bufH != c.height {
		c.drag.buf = cell.NewBufferWriter(context.Background(), c.width, c.height)
		c.drag.bufW, c.drag.bufH = c.width, c.height
	}
	c.drag.buf.SetContext(w.Context())
	_ = c.drag.buf.Clear(term.Attributes{})
	c.drawContent(c.drag.buf)

	pos := win.Position()
	off := c.WindowManagerPosition()
	pos.X += off.X
	pos.Y += off.Y

	cells := c.drag.buf.RawCells()
	veilCells(cells, pos, width, height, c.config.DropTargetAttr)
	writeCenteredLabel(cells, pos, width, height, c.dropLabel(win),
		c.config.DropTargetAttr)
	shader.Virtual(shader.Pulse(shader.PulseParams{
		Color:        c.config.DropTargetAttr.Fg,
		PeriodFrames: dragVeilPeriodFrame,
		Intensity:    dragVeilIntensity,
	}, c.config.DropTargetAttr), pos, width, height).
		Shade(int(c.drag.frame.Load()), 0, cells)

	for y, row := range cells {
		for x, cl := range row {
			w.SetCell(term.Coordinates{X: x, Y: y}, cl)
		}
	}
	return true
}

// dropLabel returns the veil message for the target window, keyed by
// the scheme of the tab it renders.
func (c *Component) dropLabel(win *browserWindow) string {
	if t, ok := browserTabAtWindow(win); ok {
		if label, ok := c.config.DropTargetLabels[t.URI().Scheme()]; ok {
			return label
		}
	}
	if label, ok := c.config.DropTargetLabels[""]; ok {
		return label
	}
	return defaultDropLabel
}

// veilCells dims the target rect by forcing the veil attributes onto
// every cell while keeping the content underneath legible.
func veilCells(
	cells [][]term.Cell, pos term.Coordinates,
	width, height int, attr term.Attributes,
) {
	forEachCell(cells, pos, width, height, func(c *term.Cell) {
		if attr.Bg != term.ColorDefault {
			c.Bg = attr.Bg
		}
		if attr.Fg != term.ColorDefault {
			c.Fg = attr.Fg
		}
	})
}

// writeCenteredLabel writes label on the middle row of the target rect,
// blanking that row first so the message stays readable over content.
func writeCenteredLabel(
	cells [][]term.Cell, pos term.Coordinates,
	width, height int, label string, attr term.Attributes,
) {
	runes := []rune(label)
	if len(runes) == 0 || len(runes) > width {
		return
	}
	y := pos.Y + height/2
	if y < 0 || y >= len(cells) {
		return
	}
	attr.Attrs |= term.AttrBold
	row := cells[y]
	start := pos.X + (width-len(runes))/2
	for i, r := range runes {
		x := start + i
		if x < 0 || x >= len(row) {
			continue
		}
		row[x] = term.NewCell(r, 1, attr)
	}
}

func forEachCell(
	cells [][]term.Cell, pos term.Coordinates,
	width, height int, fn func(*term.Cell),
) {
	for y := pos.Y; y < pos.Y+height; y++ {
		if y < 0 || y >= len(cells) {
			continue
		}
		row := cells[y]
		for x := pos.X; x < pos.X+width; x++ {
			if x < 0 || x >= len(row) {
				continue
			}
			fn(&row[x])
		}
	}
}
