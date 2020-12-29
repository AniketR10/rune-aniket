package browser

import (
	"sync"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// wraps a handlerCloser to provide unlocking a resource mutex
// on a I/O based Handler
type ioUnlock struct {
	lock sync.Locker
	h    handlerCloser
}

func newIOWaitUnlocker(h handlerCloser, lock sync.Locker) handlerCloser {
	ret := new(ioUnlock)
	ret.init(h, lock)
	return ret
}

func (h *ioUnlock) init(hc handlerCloser, lock sync.Locker) {
	h.h = hc
	h.lock = lock
}

func (h *ioUnlock) Close() error {
	return h.h.Close()
}

func (h *ioUnlock) OnUnmount() error {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.OnUnmount()
}

func (h *ioUnlock) Resize(width, height int) {
	h.lock.Unlock()
	defer h.lock.Lock()
	h.h.Resize(width, height)
}

func (h *ioUnlock) Draw(w term.Writer) {
	h.lock.Unlock()
	defer h.lock.Lock()
	h.h.Draw(w)
}

func (h *ioUnlock) Handle(ev term.Event) (exit, handled bool) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Handle(ev)
}

func (h *ioUnlock) Cursor() (pos term.Coordinates, show bool) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Cursor()
}

func (h *ioUnlock) Man() tui.Manual {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Man()
}
