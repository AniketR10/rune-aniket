package fractal

// import (
// 	"bytes"
// 	"fmt"
// 	"os"
// 	"os/exec"
// 	"os/user"
// 	"pty"
// 	"syscall"
// 	"termbox"
//
// )
//
// type Embed struct {
// 	pos           Coordinates
// 	width, height int
// 	cmd           *exec.Cmd
// 	Pty           *os.File
// 	buf           bytes.Buffer
// }
//
// func NewEmbed(x, y, width, height int, cmd *exec.Cmd) (b *Embed, err error) {
// 	b = new(Embed)
// 	return b, b.Init(x, y, width, height, cmd)
// }
//
// func (e *Embed) Init(x, y, width, height int, cmd *exec.Cmd) (err error) {
// 	e.pos.X, e.pos.Y, e.width, e.height = x, y, width, height
// 	e.cmd = cmd
//
// 	var slave, master *os.File
// 	if master, slave, err = pty.Open(); err != nil {
// 		return
// 	}
// 	e.cmd.Stdout = slave
// 	e.cmd.Stdin = slave
// 	e.cmd.Stderr = slave
// 	if e.cmd.SysProcAttr == nil {
// 		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
// 	}
// 	e.cmd.SysProcAttr.Ctty = int(slave.Fd())
// 	e.cmd.SysProcAttr.Setctty = true
// 	e.cmd.SysProcAttr.Setsid = true
//
// 	var u *user.User
// 	if u, err = user.Current(); err != nil {
// 		return
// 	}
//
// 	e.cmd.Env = append(e.cmd.Env,
// 		fmt.Sprintf("LOGNAME=%s", u.Username),
// 		fmt.Sprintf("USER=%s", u.Username),
// 		fmt.Sprintf("HOME=%s", u.HomeDir),
// 		fmt.Sprintf("TERM=%s", "xterm-256color"),
// 		//fmt.Sprintf("TERMINFO=%s", ""),
// 	)
//
// 	if err = cmd.Start(); err != nil {
// 		return
// 	}
//
// 	// TODO synchronize
// 	go func() {
// 		for {
// 			if _, err = e.buf.ReadFrom(e.Pty); err != nil {
// 				return
// 			}
// 		}
// 	}()
//
// 	// TODO design new event handler api's and internally register handler's (maybe also externally)
// 	// TODO think abouyt making it non-blocking
//
// 	// + master if err = slave.Close(); err != nil {
// 	// 	return
// 	// }
//
// 	e.Pty = master
//
// 	return e.Resize(width, height)
// }
//
// // TODO provide absrtaction on top of event that allows RawEvents too
// // alternatively provide a handler itnerface and a rawHandler
// func (e *Embed) HandleRaw(ev []byte) error {
// 	if _, err := e.Pty.Write(ev); err != nil {
// 		return err
// 	}
// 	return nil
// }
//
// func (e *Embed) GetCursor() Coordinates {
// 	// TODO
// 	return Coordinates{}
// }
//
// func (e *Embed) Resize(width, height int) error {
// 	e.width, e.height = width, height
// 	// TODO unicode support
// 	has := e.buf.Cap() - e.buf.Len()
// 	needs := width * height
// 	if has < needs {
// 		e.buf.Grow(needs - has)
// 	}
//
// 	return pty.Setsize(e.Pty, e.width, e.height)
// }
//
// func (e *Embed) Move(x, y int) error {
// 	e.pos.X, e.pos.Y = x, y
// 	return nil
// }
//
// func (e *Embed) Draw(w Writer) (err error) {
// 	outbuf := termbox.Outbuf()
// 	outbuf.Reset()
//
// 	if _, err = outbuf.Write(e.buf.Bytes()); err != nil {
// 		return
// 	}
//
// 	e.buf.Reset()
//
// 	return
// }
//
// func (e *Embed) Height() int {
// 	return e.height
// }
//
// func (e *Embed) Width() int {
// 	return e.width
// }
//
// func (e *Embed) Position() (int, int) {
// 	return e.pos.X, e.pos.Y
// }
