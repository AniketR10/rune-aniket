package handler

import (
	"bytes"
	"os"
	"os/exec"
	"pty"
	"termbox"

	"github.com/ernestrc/fractal"
)

type Embed struct {
	pos           fractal.Coordinates
	width, height int
	cmd           *exec.Cmd
	pty           *os.File
	buf           bytes.Buffer
	sync          bool
}

func NewEmbed(x, y, width, height int, cmd *exec.Cmd) (b *Embed, err error) {
	b = new(Embed)
	return b, b.Init(x, y, width, height, cmd)
}

func (e *Embed) Init(x, y, width, height int, cmd *exec.Cmd) (err error) {
	e.pos.X, e.pos.Y, e.width, e.height = x, y, width, height
	e.cmd = cmd
	// TODO not sure if necessary e.cmd.Env = append(e.cmd.Env, "TERMINFO={}")
	if e.pty, err = pty.Start(e.cmd); err != nil {
		return
	}

	return e.Resize(width, height)
}

// TODO provide absrtaction on top of event that allows RawEvents too
// alternatively provide a handler itnerface and a rawHandler
func (e *Embed) HandleRaw(ev []byte) error {
	_, err := e.buf.Write(ev)
	return err
}

func (e *Embed) SetCursor(c fractal.Cursor) error {
	// TODO translate into cursor char
	return nil
}

func (e *Embed) Resize(width, height int) error {
	e.width, e.height = width, height
	e.sync = false
	// TODO unicode support
	has := e.buf.Cap() - e.buf.Len()
	needs := width * height
	if has < needs {
		e.buf.Grow(needs - has)
	}
	return nil
}

func (e *Embed) Move(x, y int) error {
	e.pos.X, e.pos.Y = x, y
	e.sync = false
	return nil
}

func (e *Embed) Draw(w fractal.Writer) (err error) {
	if !e.sync {
		if err = pty.Setsize(e.pty, e.width, e.height); err != nil {
			return
		}
		e.sync = true
	}

	if _, err = e.pty.Write(e.buf.Bytes()); err != nil {
		return
	}
	e.buf.Reset()

	buf := make([]byte, 1024*4)

	if _, err = e.pty.Read(buf); err != nil {
		return
	}

	outbuf := termbox.Outbuf()
	outbuf.Reset()

	if _, err = outbuf.Write(buf); err != nil {
		return
	}

	//	e.buf.Reset()

	return
}

func (e *Embed) Height() int {
	return e.height
}

func (e *Embed) Width() int {
	return e.width
}

func (e *Embed) Position() (int, int) {
	return e.pos.X, e.pos.Y
}
