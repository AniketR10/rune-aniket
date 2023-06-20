package capture

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pion/mediadevices/pkg/io/video"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	timage "unstable.build/go-tui/component/image"
	"unstable.build/go-tui/term"
)

const defaultFPS = 30

var _ (tui.Component) = (*Component)(nil)

// Component is a tui.Component that draws a video source
// upon calls to Draw.
type Component struct {
	trackID, streamID  string
	cfg                timage.Config
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
	reader video.Reader, cfg timage.Config,
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
	cfg timage.Config,
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
	go c.consumeVideoSource(cadence)
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
	case _, _ = <-c.consumeVideoActive:
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

	var width, height int
	for {
		select {
		case resize := <-c.resize:
			width = resize.width
			height = resize.height
			c.log(log.TraceLevel, "resize received. width=%d height=%d", width, height)
		case <-ticker.C:
			err := c.consumeFrame(width, height)
			c.log(log.TraceLevel, "read new frame "+
				"width=%v, height=%v, err=%s", width, height, err)
			if err != nil {
				continue
			}
			if err := c.interrupter.Interrupt(); err != nil {
				c.log(log.WarnLevel, "interrupt error: %s", err)
				continue
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Component) consumeFrame(width, height int) error {
	img, release, err := c.reader.Read()
	if err != nil {
		return fmt.Errorf("video read frame: %v", err)
	}
	defer release()

	c.rmu.Lock()
	defer c.rmu.Unlock()

	timage.Encode(&c.buf, width, height, img, c.cfg)
	return nil
}

func (c *Component) log(level log.Level, msg string, args ...interface{}) {
	log.WithFields(log.Fields{
		"trackID":  c.trackID,
		"streamID": c.streamID,
	}).Logf(level, msg, args...)
}

type resize struct {
	width, height int
}
