package plugin

import (
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/vte"
)

// Option represents a Handler configuration option.
type Option func(*handlerConfig)

// WithVTEConfig instructs Handler to use
// the given vte.Config as the underlying vte.Handler config.
func WithVTEConfig(cfg vte.Config) Option {
	return func(hcfg *handlerConfig) {
		hcfg.cfg = cfg
	}
}

// WithFrame returns an option that configures
// whether Handler draws a divider between the top
// bar and the vte.
func WithFrame(frame bool) Option {
	return func(cfg *handlerConfig) {
		cfg.frame = frame
	}
}

// WithFrameCharSet returns an option that configures
// Handler to use the given character set to draw
// a divider between the top bar and the vte. It's a no-op
// if frame is set to false.
func WithFrameCharSet(charSet component.FrameCharSet) Option {
	return func(cfg *handlerConfig) {
		cfg.frameCharSet = charSet
	}
}

// WithFrameAttr returns an option that configures
// the divider attributes.
func WithFrameAttr(attr term.Attributes) Option {
	return func(cfg *handlerConfig) {
		cfg.frameAttr = attr
	}
}

type handlerConfig struct {
	cfg          vte.Config
	frame        bool
	frameCharSet component.FrameCharSet
	frameAttr    term.Attributes
}

func defaultConfig() handlerConfig {
	return handlerConfig{
		cfg:          vte.DefaultConfig(),
		frame:        true,
		frameCharSet: component.FrameCharSetDefault(),
		frameAttr:    term.Attributes{},
	}
}
