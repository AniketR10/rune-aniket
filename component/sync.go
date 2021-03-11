package component

import (
	"sync"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

type csync struct {
	mu sync.Locker
	c  tui.Component
}

// Sync wraps a tui.Component to provide access synchronization with mu.
func Sync(mu sync.Locker, c tui.Component) tui.Component {
	return csync{mu: mu, c: c}
}

func (s csync) Resize(width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c.Resize(width, height)
}

func (s csync) Draw(w term.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c.Draw(w)
}
