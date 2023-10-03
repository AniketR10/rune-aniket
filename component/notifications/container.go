package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"

	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// Level represents the notification level.
type Level uint8

const (
	// LevelError signals an unexpected error.
	LevelError Level = iota
	// LevelWarn signals an expected error.
	LevelWarn
	// LevelInfo signals an informational message.
	LevelInfo
	// LevelSuccess signals a message of success.
	LevelSuccess
)

// Config configures Container.
type Config struct {
	// AutoClose determines how long it takes for a given notification to auto-close.
	AutoClose time.Duration
	// ProgressBar switches an auto-close progress bar on or off.
	ProgressBar bool
	// Width determines the width of each notification in columns.
	Width int
	// Attributes of the notification text.
	Attributes term.Attributes
	// BackgroundAttributes of the notification text.
	BackgroundAttributes term.Attributes
	// FrameCharSet of the notification frame if Frame is enabled.
	// This is optional. If not set, component.FrameCharSetDefault is used.
	FrameCharSet component.FrameCharSet
	// Interrupt is needed to asynchronously update the UI. This is optional.
	Interrupter term.Interrupter
}

// Container renders an inner tui.Component and overlays any notifications that were
// posted via Notify.
type Container struct {
	cfg   Config
	inner tui.Component

	mu    sync.RWMutex
	list  component.ResponsiveList
	vlist component.Virtual

	ctx       context.Context
	cancelCtx func()
}

// New allocates storage for a new instance of Container and initializes it with
// inner. See Containerfor more details
func New(inner tui.Component, cfg Config) *Container {
	ret := new(Container)
	ret.Init(inner, cfg)
	return ret
}

// Initi initializes this Container with inner and cfg. It panics if cfg.Width <= 0.
func (n *Container) Init(inner tui.Component, cfg Config) {
	if cfg.Width <= 0 {
		panic(fmt.Sprintf("notifications.Container with invalid width: %v", cfg.Width))
	}
	if cfg.AutoClose == 0 {
		panic(fmt.Sprintf("notifications.Container with invalid close timeout: %v", cfg.AutoClose))
	}
	if cfg.FrameCharSet == (component.FrameCharSet{}) {
		cfg.FrameCharSet = component.FrameCharSetDefault()
	}
	n.inner = inner
	n.cfg = cfg
	n.list.Init()
	n.vlist.C = &n.list
	n.ctx, n.cancelCtx = context.WithCancel(context.Background())
}

// Draw satisfies tui.Component.
func (n *Container) Draw(w term.Writer) {
	n.inner.Draw(w)

	n.mu.RLock()
	defer n.mu.RUnlock()

	n.vlist.Draw(w)
}

// Resize satisfies tui.Component.
func (n *Container) Resize(width, height int) {
	n.inner.Resize(width, height)

	n.mu.Lock()
	defer n.mu.Unlock()

	effectiveWidth := n.cfg.Width
	offset := width - n.cfg.Width
	if offset < 0 {
		offset = 0
		effectiveWidth = width
	}

	n.vlist.Move(term.Coordinates{X: offset, Y: 0})
	n.vlist.Resize(effectiveWidth, height)
}

func (n *Container) Notify(level Level, msg string) {
	var comp component.Responsive
	if n.cfg.ProgressBar {
		comp = newNotification(level, msg, n.cfg)
	} else {
		comp = newString(n.cfg, msg)
	}

	ctx, cancel := context.WithTimeout(n.ctx, n.cfg.AutoClose)

	go func() {
		const fps = 10
		cadence := time.Duration(int(time.Second) / fps)
		ticker := time.NewTicker(cadence)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = n.cfg.Interrupter.Interrupt()
			case <-ctx.Done():
				return
			}
		}

	}()

	n.mu.Lock()
	defer n.mu.Unlock()

	el := n.list.PushFront(comp)

	go func() {
		defer cancel()

		<-ctx.Done()

		n.mu.Lock()
		defer n.mu.Unlock()

		n.list.Remove(el)

		if n.cfg.Interrupter != nil {
			_ = n.cfg.Interrupter.Interrupt()
		}
	}()
}

// Close cancells all pending notifications.
func (n *Container) Close() error {
	n.cancelCtx()
	return nil
}
