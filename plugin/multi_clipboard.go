package plugin

import (
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	
	"unstable.build/go-tui/text"
)

var (
	_ ClipboardRegister = (*multiClipboard)(nil)
	_ io.Closer         = (*multiClipboard)(nil)
)

type multiClipboard struct {
	data        string
	lastUpdated time.Time
	idx         int
	clipboards  map[int]ClipboardRegister
}

func newMultiClipboard() *multiClipboard {
	ret := new(multiClipboard)
	ret.clipboards = make(map[int]ClipboardRegister)
	return ret
}

func (c *multiClipboard) add(r ClipboardRegister) string {
	c.clipboards[c.idx] = r
	res := strconv.Itoa(c.idx)
	c.idx++
	return res
}
func (c *multiClipboard) remove(strIdx string) {
	idx, _ := strconv.Atoi(strIdx)
	delete(c.clipboards, idx)
}

func (c *multiClipboard) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "plugin.multiClipboard").
		Logf(level, msg, args...)
}

// Paste returns the data from the last-updated clipboard.
func (c *multiClipboard) Paste() (ret string, updated time.Time, retErr error) {
	type pasteResult struct {
		data    string
		updated time.Time
	}
	var wg sync.WaitGroup
	results := make([]pasteResult, len(c.clipboards))
	wg.Add(len(c.clipboards))
	var i int
	for _, reg := range c.clipboards {
		go func(idx int, reg ClipboardRegister) {
			defer wg.Done()
			data, updated, err := reg.Paste()
			if err != nil {
				c.log(log.WarnLevel, "Paste error: %v", err)
				return
			}
			results[idx] = pasteResult{data: data, updated: updated}
		}(i, reg)
		i++
	}
	wg.Wait()

	ret = c.data
	lastUpdated := c.lastUpdated
	for _, result := range results {
		if result.updated.After(lastUpdated) {
			ret = result.data
			lastUpdated = result.updated
		}
	}
	updated = lastUpdated
	return
}

// Copy copies the data to all clipboards, including itself
func (c *multiClipboard) Copy(data string, timestamp time.Time) error {
	if len(c.clipboards) == 0 {
		c.add(&textRegister{r: text.NewInMemoryClipboard()})
	}
	var wg sync.WaitGroup
	wg.Add(len(c.clipboards))
	for _, clip := range c.clipboards {
		go func(clip ClipboardRegister) {
			defer wg.Done()
			// since we have the fallback to using
			// multiClipboard inmemory data, we
			// can safely ignore individual Copy errors
			err := clip.Copy(data, timestamp)
			if err != nil {
				c.log(log.WarnLevel, "Copy error: %v", err)
			}
		}(clip)
	}
	wg.Wait()
	c.data = data
	c.lastUpdated = timestamp
	return nil
}

func (c *multiClipboard) Close() (ret error) {
	for _, clip := range c.clipboards {
		if closer, ok := clip.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				ret = multierr.Append(ret, err)
			}
		}
	}
	return ret
}
