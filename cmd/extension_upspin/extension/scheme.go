// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package extension

import (
	"bytes"
	"context"
	stdErrors "errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"syscall"

	multierr "github.com/ernestrc/go-multierror"
	blupspin "github.com/unstablebuild/blue/upspin"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
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
	configKeys = []string{"username", "keyserver",
		"dirserver", "storeserver", "packing", "secrets", "tlscerts"}
)

type scheme struct {
	uri    workspaceapi.URI
	client *upspinClient
	files  map[uintptr]workspaceapi.File
	fd     uintptr // next fd
}

func newScheme(ctx context.Context, config config.Config, uri workspaceapi.URI) (
	schemeapi.Scheme, error,
) {
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

// notes on upspin configuration, taken from upspin docs:
// Any endpoints (keyserver, dirserver, storeserver) not set in the data for
// the config will be set to the "unassigned" transport and an empty network
// address, except keyserver which defaults to "remote,key.upspin.io:443".
// If an endpoint is specified without a transport it is assumed to be
// the address component of a remote endpoint.
// If a remote endpoint is specified without a port in its address component
// the port is assumed to be 443.
// The default value for secrets is "$HOME/.ssh/$USERNAME".
// The special value "none" indicates there are no secrets to load;
// The default value for tlscerts is the empty string,
// in which case just the system roots are used.
// The default value for packing is "ee".
func configToReader(c config.Config) (ret io.Reader, retErr error) {
	var buf bytes.Buffer
	for _, key := range configKeys {
		value, err := c.GetString(key)
		if err != nil && err != config.ErrNotFound {
			retErr = multierr.Append(retErr, fmt.Errorf("%s: %v", key, err))
		}
		if value != "" {
			buf.WriteString(fmt.Sprintf("%s: %s \n", key, value))
		}
	}
	if retErr != nil {
		return
	}
	if buf.Len() == 0 {
		return
	}
	ret = &buf
	return
}

func initUpspinConfig(c config.Config) (ret upspin.Config, retErr error) {
	r, err := configToReader(c)
	if err != nil {
		retErr = multierr.Append(retErr,
			fmt.Errorf("Failed to load provided config via workspace.upspin: %v", err))
		// fallback to default config
	}
	ret, err = upcfg.InitConfig(r)
	if err != nil {
		if r == nil {
			retErr = multierr.Append(retErr, fmt.Errorf("Failed to load default config at $HOME/upspin/config: %v", err))
		} else {
			retErr = multierr.Append(retErr, fmt.Errorf("upspin.InitConfig: %v", err))
		}
	}
	return
}

func (s *scheme) init(config config.Config, uri workspaceapi.URI) error {
	cfg, err := initUpspinConfig(config)
	if err != nil {
		return err
	}

	// initialize and register transports
	transports.Init(cfg)
	s.uri = uri

	s.client = newUpspinClient(upclient.New(cfg))
	s.files = make(map[uintptr]workspaceapi.File)
	return nil
}

func (s *scheme) expandPath(path string) (string, error) {
	return workspaceapi.ExpandPathWithURI(path, s.uri)
}

func (s *scheme) URI(path string) (workspaceapi.URI, error) {
	return workspace.WorkspaceURI(s.uri, path)
}

// NewFile simply calls Open under the hood as upspin files are generally not cached in memory.
func (s *scheme) NewFile(fd uintptr, path string) workspaceapi.File {
	return s.files[fd]
}

func (s *scheme) Open(path string, flag int, mode os.FileMode) (workspaceapi.File, *workspaceapi.Error) {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return nil, workspaceapi.NopError(err)
	}
	f, err := blupspin.Open(s.client, uname, flag)
	if err != nil {
		return nil, mapUpspinError(err)
	}
	if flag&os.O_TRUNC != 0 {
		err := f.Truncate(0)
		if err != nil {
			return nil, mapUpspinError(err)
		}
	}
	path, _ = s.expandPath(path)
	s.fd++
	ret := &fileAdapter{
		s:      s,
		client: s.client,
		file:   f,
		path:   path,
		fd:     s.fd,
	}
	if flag&os.O_CREATE != 0 {
		// make sure entry is created
		err := ret.Sync()
		if err != nil {
			return nil, mapUpspinError(err)
		}
	} else {
		ret.lastEntry, err = s.client.Lookup(uname, true)
		if err != nil {
			return nil, mapUpspinError(err)
		}
	}
	s.files[ret.Fd()] = ret
	return ret, nil
}

func (s *scheme) Remove(path string) error {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return err
	}
	err = s.client.Delete(uname)
	if err != nil {
		return mapUpspinError(err).ToError()
	}
	return nil
}

func (s *scheme) Rename(oldpath, newpath string) error {
	uold, err := s.makeUpspinPathname(oldpath)
	if err != nil {
		return err
	}
	unew, err := s.makeUpspinPathname(newpath)
	if err != nil {
		return err
	}
	// Workaround around Rename failing with 'item already exists'
	// error if target file is present.
	// NOTE for now we have no mechanism to recover
	// or even detect if a backup is present
	backup := upspin.PathName(fmt.Sprintf("%s.backup", unew))

	_, err = s.client.Rename(unew, backup)
	// ignore error if there's already a backup to avoid
	// always having an extra rountrip to delete first
	if err != nil && !errors.Is(errors.NotExist, err) {
		err = mapUpspinError(err).ToError()
		return err
	}

	_, err = s.client.Rename(uold, unew)
	if err != nil {
		err = mapUpspinError(err).ToError()
		// restore backup
		_, rerr := s.client.Rename(backup, unew)
		if rerr != nil {
			err = multierr.Append(err,
				fmt.Errorf("Critical: Rename recover from backup %s "+
					"failed. Must restore manually: %v",
					backup, rerr))
		}
		return err
	}

	_ = s.client.Delete(backup)
	return nil
}

func (s *scheme) Stat(path string) (os.FileInfo, error) {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return nil, err
	}
	entry, err := s.client.Lookup(uname, true)
	if err != nil {
		return nil, mapUpspinError(err).ToError()
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
		return nil, mapUpspinError(err).ToError()
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
		return "", mapUpspinError(err).ToError()
	}
	if entry.Link == "" {
		return "", errors.E("not a link")
	}
	return string(entry.Link), nil
}

func (s *scheme) ReadDir(name string) ([]os.DirEntry, error) {
	info, err := s.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, stdErrors.New("not a directory")
	}

	uname, err := s.makeUpspinPathname(name)
	if err != nil {
		return nil, err
	}

	pattern := fmt.Sprintf("%s/*", string(uname))
	entries, err := s.client.Glob(pattern)
	if err != nil {
		return nil, mapUpspinError(err).ToError()
	}

	ret := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		u, err := workspaceapi.ParseURI("upspin://" + string(entry.Name))
		if err != nil {
			return nil, err
		}
		name = workspaceapi.RelPath(s.uri, u)
		ret = append(ret, dirEntryAdapter{name: name, s: s, entry: entry})
	}
	return ret, nil
}

func (s *scheme) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return 0, errExecute
}

func (s *scheme) Signal(workspaceapi.Pid, syscall.Signal) error {
	return errExecute
}

func (s *scheme) NewPty(context.Context) (workspaceapi.Pty, error) {
	return workspaceapi.Pty{}, errExecute
}

func (s *scheme) SetPtySize(workspaceapi.Pty, int, int) error {
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

func mapUpspinError(err error) *workspaceapi.Error {
	return &workspaceapi.Error{
		Err:          err,
		IsPermission: errors.Is(errors.Permission, err),
		IsExist:      errors.Is(errors.Exist, err),
		IsNotExist:   errors.Is(errors.NotExist, err),
	}
}

type dirEntryAdapter struct {
	name  string
	entry *upspin.DirEntry
	s     *scheme
}

func (d dirEntryAdapter) Info() (os.FileInfo, error) {
	entry, err := d.s.client.Lookup(d.entry.Name, true)
	if err != nil {
		return nil, mapUpspinError(err).ToError()
	}
	return entryAdapter{entry}, nil
}

func (d dirEntryAdapter) IsDir() bool {
	return d.entry.IsDir()
}

func (d dirEntryAdapter) Name() string {
	return d.name
}

func (d dirEntryAdapter) Type() os.FileMode {
	if d.entry.IsDir() {
		return os.ModeDir
	}
	if d.entry.Attr&upspin.AttrLink != 0 {
		return os.ModeSymlink
	}
	return 0
}
