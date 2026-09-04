// Copyright (C) 2017-2026 Unstable Build, LLC
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

package capture

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pion/mediadevices/pkg/io/video"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/cell"
	"unstable.build/rune/component"
	"unstable.build/rune/component/asciiart"
	"unstable.build/rune/debug"
)

const defaultFPS = 30

var _ (tui.Component) = (*Component)(nil)

// Component is a tui.Component that draws a video source
// upon calls to Draw.
type Component struct {
	trackID, streamID  string
	cfg                asciiart.Config
	interrupter        term.Interrupter
	reader             video.Reader
	ctx                context.Context
	cancelCtx          func()
	resize             chan resize
	consumeVideoActive chan struct{}

	rmu    sync.RWMutex
	buf    cell.Buffer
	scroll component.Scroll
}

// NewComponent allocates storage for a new Component and initializes it.
// See Init for more details.
func NewComponent(
	trackID, streamID string,
	interrupter term.Interrupter, fps int,
	reader video.Reader, cfg asciiart.Config,
) *Component {
	ret := new(Component)
	ret.Init(trackID, streamID, interrupter, fps, reader, cfg)
	return ret
}

// Init initializes this component with the given interrupter, fps
// video source and ASCII image encoding configuration.
func (c *Component) Init(
	trackID, streamID string,
	interrupter term.Interrupter, fps int, reader video.Reader,
	cfg asciiart.Config,
) {
	c.trackID, c.streamID = trackID, streamID
	c.cfg = cfg
	c.interrupter = interrupter
	c.reader = reader
	c.ctx, c.cancelCtx = context.WithCancel(context.Background())
	c.resize = make(chan resize)
	c.consumeVideoActive = make(chan struct{})

	c.buf.Init()
	c.scroll.Init(&c.buf)

	if fps == 0 {
		fps = defaultFPS
	}

	cadence := time.Duration(int(time.Second) / fps)
	go debug.CapturePanicReport(func() {
		c.consumeVideoSource(cadence)
	})
}

// Draw satisfies tui.Component.
func (c *Component) Draw(writer term.Writer) {
	c.rmu.RLock()
	defer c.rmu.RUnlock()

	c.scroll.Draw(writer)
}

// Resize satisfies tui.Component.
func (c *Component) Resize(width, height int) {
	c.log(log.DebugLevel, "resizing to width=%d height=%d", width, height)
	c.scroll.Resize(width, height)

	select {
	case c.resize <- resize{width, height}:
	case <-c.consumeVideoActive:
		// do not block main event loop if consumeVideoSource dies
		return
	}
}

// Close closes all resources associated with this Component.
func (c *Component) Close() error {
	c.cancelCtx()
	return nil
}

func (c *Component) consumeVideoSource(cadence time.Duration) {
	defer c.log(log.InfoLevel, "done consuming from video source")
	c.log(log.InfoLevel, "consuming from video source")

	ticker := time.NewTicker(cadence)
	defer ticker.Stop()
	defer close(c.consumeVideoActive)

	var width, height, frameWidth, frameHeight int
	var err error
	for {
		select {
		case resize := <-c.resize:
			width = resize.width
			height = resize.height
			// pre-calculate aspect ratio to avoid
			// Encode having to calculate it for each frame.
			if c.cfg.MaintainAspectRatio && frameWidth != 0 && frameHeight != 0 {
				width, height = asciiart.ResizeMaintainAspectRatio(
					frameWidth, frameHeight, width, height)
			}
			c.log(log.TraceLevel, "resize received. width=%d height=%d", width, height)
		case <-ticker.C:
			frameWidth, frameHeight, err = c.consumeFrame(width, height)
			c.log(log.TraceLevel, "read new frame "+
				"width=%v, height=%v, err=%s", width, height, err)
			if err != nil {
				continue
			}
			if err := c.interrupter.Interrupt(c.ctx); err != nil {
				c.log(log.WarnLevel, "interrupt error: %s", err)
				continue
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Component) consumeFrame(width, height int) (
	frameWidth, frameHeight int, err error,
) {
	img, release, err := c.reader.Read()
	if err != nil {
		err = fmt.Errorf("video read frame: %v", err)
		return
	}
	defer release()

	c.rmu.Lock()
	defer c.rmu.Unlock()

	// aspect ratio is pre-calculated on resize to avoid
	// Encode having to calculate it for each frame.
	cfg := c.cfg
	cfg.MaintainAspectRatio = false
	asciiart.Encode(&c.buf, width, height, img, cfg)

	frameWidth = img.Bounds().Dx()
	frameHeight = img.Bounds().Dy()
	return
}

func (c *Component) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		"trackID":  c.trackID,
		"streamID": c.streamID,
	}).Logf(level, msg, args...)
}

type resize struct {
	width, height int
}
