package plugin

import (
	"io"
	"strconv"

	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/text"
)

var (
	_ ClipboardRegister = (*multiClipboard)(nil)
	_ io.Closer         = (*multiClipboard)(nil)
)

type multiClipboard struct {
	idx        int
	clipboards map[int]ClipboardRegister
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

func (c *multiClipboard) Paste() (string, error) {
	for _, v := range c.clipboards {
		return v.Paste()
	}
	return "", nil
}

func (c *multiClipboard) Copy(data string) (ret error) {
	if len(c.clipboards) == 0 {
		c.add(&textRegister{r: text.NewInMemoryClipboard()})
	}
	for _, clip := range c.clipboards {
		if err := clip.Copy(data); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return
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
