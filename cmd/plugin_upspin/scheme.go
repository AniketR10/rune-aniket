package main

import (
	stdErrors "errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"syscall"

	blupspin "github.com/ernestrc/blue/upspin"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
	upclient "upspin.io/client"
	upcfg "upspin.io/config"
	"upspin.io/errors"
	"upspin.io/transports"
	"upspin.io/upspin"
)

const upspinScheme = "upspin"

var (
	errExecute = stdErrors.New("cannot execute commands on upspin server")
)

type scheme struct {
	uri    workspace.URI
	client *upspinClient
}

func newScheme(config config.Config, uri workspace.URI) (workspace.Scheme, error) {
	if uri.Scheme() != upspinScheme {
		return nil, stdErrors.New("invalid scheme")
	}
	ret := new(scheme)
	err := ret.init(config, uri)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *scheme) init(config config.Config, uri workspace.URI) error {
	// NOTE: assume for now default upspin config dir/file is used
	// which is $HOME/upspin/config
	cfg, err := upcfg.InitConfig(nil)
	if err != nil {
		return fmt.Errorf("upspin.InitConfig: %v", err)
	}

	// initialize and register transports
	transports.Init(cfg)
	s.uri = uri

	s.client = newUpspinClient(upclient.New(cfg))
	return nil
}

func (s *scheme) expandPath(path string) (string, error) {
	return workspace.ExpandPathWithURI(path, s.uri)
}

func (s *scheme) URI(path string) (workspace.URI, error) {
	absPath, err := s.expandPath(path)
	if err != nil {
		return workspace.URI{}, err
	}
	if err != nil {
		return workspace.URI{}, err
	}
	uriStr := fmt.Sprintf("upspin://%s@%s%s", s.uri.User(), s.uri.Host(), absPath)
	return workspace.ParseURI(uriStr)
}

func (s *scheme) Open(path string, flag int, perm os.FileMode) (workspace.File, *workspace.Error) {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return nil, workspace.NopError(err)
	}
	f, err := blupspin.Open(s.client, uname, flag)
	if err != nil {
		return nil, mapUpspinError(err)
	}
	if flag&os.O_CREATE != 0 {
		// make sure entry is created
		err := f.Sync()
		if err != nil {
			return nil, mapUpspinError(err)
		}
	}
	return fileAdapter{client: s.client, file: f}, nil
}

func (s *scheme) Remove(path string) error {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return err
	}
	err = s.client.Delete(uname)
	if err != nil {
		return mapUpspinError(err)
	}
	return nil
}

// TODO review this
func (s *scheme) Rename(oldpath, newpath string) error {
	uold, err := s.makeUpspinPathname(oldpath)
	if err != nil {
		return err
	}
	unew, err := s.makeUpspinPathname(newpath)
	if err != nil {
		return err
	}
	// NOTE for now we have no mechanism to recover
	// or even detect if a backup is present
	backup := upspin.PathName(fmt.Sprintf("%s.backup", unew))

	_, err = s.client.Rename(unew, backup)
	// ignore error if there's already a backup to avoid
	// always having an extra rountrip to delete first
	if err != nil && !errors.Is(errors.Exist, err) {
		return mapUpspinError(err)
	}

	_, err = s.client.Rename(uold, unew)
	if err != nil {
		return mapUpspinError(err)
	}

	go s.client.Delete(backup)
	return nil
}

func (s *scheme) Stat(path string) (os.FileInfo, error) {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return nil, err
	}
	entry, err := s.client.Lookup(uname, true)
	if err != nil {
		return nil, mapUpspinError(err)
	}
	return entryAdapter{entry: entry}, nil
}

func (s *scheme) Lstat(path string) (os.FileInfo, error) {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return nil, err
	}
	entry, err := s.client.Lookup(uname, false)
	if err != nil {
		return nil, mapUpspinError(err)
	}
	return entryAdapter{entry: entry}, nil
}

func (s *scheme) ReadLink(path string) (string, error) {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return "", err
	}
	entry, err := s.client.Lookup(uname, false)
	if err != nil {
		return "", mapUpspinError(err)
	}
	return string(entry.Link), nil
}

func (s *scheme) Command(name string, arg ...string) (workspace.Pid, error) {
	return 0, errExecute
}

func (s *scheme) Start(workspace.Pid) error {
	return errExecute
}

func (s *scheme) Signal(workspace.Pid, syscall.Signal) error {
	return errExecute
}

func (s *scheme) StderrPipe(workspace.Pid) (io.ReadCloser, error) {
	return nil, errExecute
}

func (s *scheme) StdinPipe(workspace.Pid) (io.WriteCloser, error) {
	return nil, errExecute
}

func (s *scheme) StdoutPipe(workspace.Pid) (io.ReadCloser, error) {
	return nil, errExecute
}

func (s *scheme) Wait(workspace.Pid) error {
	return errExecute
}

func (s *scheme) Close() error {
	return nil
}

func (s *scheme) makeUpspinPathname(path string) (upspin.PathName, error) {
	// first check if it's an upspin name already
	// usually as a return of fileAdapter.Name()
	u, err := url.Parse("upspin://" + path)
	if err == nil && u.Host != "" && u.User != nil && u.User.Username() != "" {
		return upspin.PathName(path), nil
	}

	absPath, err := s.expandPath(path)
	if err != nil {
		return "", err
	}

	uriStr := fmt.Sprintf("%s@%s%s",
		s.uri.User(), s.uri.Host(), absPath)

	return upspin.PathName(uriStr), nil
}

func mapUpspinError(err error) *workspace.Error {
	return &workspace.Error{
		Err:          err,
		IsPermission: errors.Is(errors.Permission, err),
		IsExist:      errors.Is(errors.Exist, err),
		IsNotExist:   errors.Is(errors.NotExist, err),
	}
}
