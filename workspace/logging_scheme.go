package workspace

import (
	"io"
	"os"
	"syscall"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"
)

// LoggingScheme wraps a SchemeFunc with a constructor
// that wraps the underlying scheme with a scheme that
// logs every method call.
func LoggingScheme(scheme string, fn SchemeFunc) SchemeFunc {
	return func(cfg config.Config, uri workspaceapi.URI) (Scheme, error) {
		other, err := fn(cfg, uri)
		if err != nil {
			log.Errorf("SchemeFunc error: %s", err)
			return nil, err
		}
		return loggingScheme{
			scheme: scheme,
			uri:    uri,
			other:  other,
		}, nil
	}
}

type loggingScheme struct {
	scheme string
	uri    workspaceapi.URI
	other  Scheme
}

func (t loggingScheme) trace(msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "LoggingScheme").
		WithField("URI", t.uri.String()).
		WithField("scheme", t.scheme).
		Tracef(msg, args...)
}

func (t loggingScheme) Command(name string, arg ...string) (p workspaceapi.Pid, err error) {
	t.trace("Command(%q, %v)", name, arg)
	p, err = t.other.Command(name, arg...)
	t.trace("Command(%q, %v): %d, %v", name, arg, p, err)
	return
}

func (t loggingScheme) Start(p workspaceapi.Pid) (err error) {
	t.trace("Start(%d)", p)
	err = t.other.Start(p)
	t.trace("Start(%d): %v", p, err)
	return
}

func (t loggingScheme) Signal(p workspaceapi.Pid, s syscall.Signal) (err error) {
	t.trace("Signal(%d, %d)", p, s)
	err = t.other.Signal(p, s)
	t.trace("Signal(%d, %d): %v", p, s, err)
	return
}

func (t loggingScheme) StderrPipe(p workspaceapi.Pid) (ret io.ReadCloser, err error) {
	t.trace("StderrPipe(%d)", p)
	ret, err = t.other.StderrPipe(p)
	t.trace("StderrPipe(%d): %v", p, err)
	return
}

func (t loggingScheme) StdinPipe(p workspaceapi.Pid) (ret io.WriteCloser, err error) {
	t.trace("StdinPipe(%d)", p)
	ret, err = t.other.StdinPipe(p)
	t.trace("StdinPipe(%d): %v", p, err)
	return
}

func (t loggingScheme) StdoutPipe(p workspaceapi.Pid) (ret io.ReadCloser, err error) {
	t.trace("StdoutPipe(%d)", p)
	ret, err = t.other.StdoutPipe(p)
	t.trace("StdoutPipe(%d): %v", p, err)
	return
}

func (t loggingScheme) Wait(p workspaceapi.Pid) (err error) {
	t.trace("Wait(%d)", p)
	err = t.other.Wait(p)
	t.trace("Wait(%d): %v", p, err)
	return
}

func (t loggingScheme) URI(path string) (ret workspaceapi.URI, err error) {
	t.trace("URI(%q)", path)
	ret, err = t.other.URI(path)
	t.trace("URI(%q): %q %v", path, ret.String(), err)
	return
}

func (t loggingScheme) Open(path string, flag int, perm os.FileMode) (
	ret workspaceapi.File, err *workspaceapi.Error,
) {
	t.trace("Open(%q, %d, %d)", path, flag, perm)
	ret, err = t.other.Open(path, flag, perm)
	t.trace("Open(%q, %d, %d): %#v, %#v", path, flag, perm, ret, err)
	return
}

func (t loggingScheme) Remove(path string) (err error) {
	t.trace("Remove(%q)", path)
	err = t.other.Remove(path)
	t.trace("Remove(%q): %v", path, err)
	return
}

func (t loggingScheme) Rename(old, new string) (err error) {
	t.trace("Rename(%q, %q)", old, new)
	err = t.other.Rename(old, new)
	t.trace("Rename(%q, %q): %v", old, new, err)
	return
}

func (t loggingScheme) Stat(path string) (ret os.FileInfo, err error) {
	t.trace("Stat(%q)", path)
	ret, err = t.other.Stat(path)
	t.trace("Stat(%q): %#v, %v", path, ret, err)
	return
}

func (t loggingScheme) Lstat(path string) (ret os.FileInfo, err error) {
	t.trace("Lstat(%q)", path)
	ret, err = t.other.Lstat(path)
	t.trace("Lstat(%q): %#v, %v", path, ret, err)
	return
}

func (t loggingScheme) ReadLink(path string) (ret string, err error) {
	t.trace("ReadLink(%q)", path)
	ret, err = t.other.ReadLink(path)
	t.trace("ReadLink(%q): %q, %v", path, ret, err)
	return
}
func (t loggingScheme) NewPty() (ret workspaceapi.Pty, err error) {
	t.trace("NewPty()")
	ret, err = t.other.NewPty()
	t.trace("NewPty(): %q, %v", ret, err)
	return
}

func (t loggingScheme) SetPtySize(p workspaceapi.Pty, width, height int) (err error) {
	t.trace("SetPtySize(%v, %d, %d)", p, width, height)
	err = t.other.SetPtySize(p, width, height)
	t.trace("SetPtySize(%v, %d, %d): %v", p, width, height, err)
	return
}

func (t loggingScheme) ReadDir(name string) (
	ret []os.DirEntry, err error,
) {
	t.trace("ReadDir(%s)", name)
	ret, err = t.other.ReadDir(name)
	t.trace("ReadDir(%s): %v, %v", name, ret, err)
	return
}

func (t loggingScheme) Close() (err error) {
	t.trace("Close")
	err = t.other.Close()
	t.trace("Close: %q", err)
	return
}
