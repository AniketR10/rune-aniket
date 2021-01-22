package browser

import (
	"sync"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

// wraps a handlerCloser to provide unlocking a resource mutex while waiting
// for a I/O based Handler to respond.
type ioUnlockHandler struct {
	lock sync.Locker
	h    *handler.Client
}

func newIOWaitUnlockHandler(h *handler.Client, lock sync.Locker) handlerCloser {
	ret := new(ioUnlockHandler)
	ret.init(h, lock)
	return ret
}

func (h *ioUnlockHandler) init(hc *handler.Client, lock sync.Locker) {
	h.h = hc
	h.lock = lock
}

func (h *ioUnlockHandler) Close() error {
	return h.h.Close()
}

func (h *ioUnlockHandler) OnUnmount() error {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.OnUnmount()
}

func (h *ioUnlockHandler) Resize(width, height int) {
	h.lock.Unlock()
	defer h.lock.Lock()
	h.h.Resize(width, height)
}

func (h *ioUnlockHandler) Draw(w term.Writer) {
	h.lock.Unlock()
	defer h.lock.Lock()
	h.h.Draw(w)
}

func (h *ioUnlockHandler) Handle(ev term.Event) (exit, handled bool) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Handle(ev)
}

func (h *ioUnlockHandler) Cursor() (pos term.Coordinates, show bool) {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Cursor()
}

func (h *ioUnlockHandler) Man() tui.Manual {
	h.lock.Unlock()
	defer h.lock.Lock()
	return h.h.Man()
}
