package api

// ChanWatcher returns a Watcher that simply returns
// ch when Watch is called.
func ChanWatcher(ch chan error) Watcher {
	return waitCh(ch)
}

type waitCh chan error

func (w waitCh) Watch() chan error {
	return w
}
