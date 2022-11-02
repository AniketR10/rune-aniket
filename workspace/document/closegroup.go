package document

import "sync"

// closeGroup is a sync.WaitGroup in which
// wg.Add and wg.Wait can be called concurrently from
// different goroutines.
type closeGroup struct {
	mu         sync.Mutex
	wg         sync.WaitGroup
	waitClosed bool
}

func (wg *closeGroup ) AddOne() (func(), bool) {
	wg.mu.Lock()
	if wg.waitClosed {
		wg.mu.Unlock()
		return nil, false
	}
	wg.wg.Add(1)
	wg.mu.Unlock()
	return wg.wg.Done, true
}

func (wg *closeGroup ) Close() bool {
	wg.mu.Lock()
	if wg.waitClosed {
		wg.mu.Unlock()
		return false
	}
	wg.waitClosed = true
	wg.mu.Unlock()
	wg.wg.Wait()
	return true
}
