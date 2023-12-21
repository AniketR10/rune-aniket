package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// WithLoggin wraps comp to log calls to Resize and Draw using the provided logger.
func WithLogging(comp tui.Component, logger func(string, ...any)) tui.Component {
	if logger == nil {
		panic("logger cannot be nil")
	}
	return withLogging{comp: comp, logger: logger}
}

type withLogging struct {
	comp   tui.Component
	logger func(string, ...any)
}

func (l withLogging) Resize(width, height int) {
	l.logger("Resize(%p): width=%d, height=%d", l.comp, width, height)
	l.comp.Resize(width, height)
}

func (l withLogging) Draw(w term.Writer) {
	l.logger("Draw(%p)", l.comp)
	l.comp.Draw(w)
}
