package main

import (
	"bytes"
	"context"
	stdErrors "errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"github.com/ernestrc/blue/iterator"
	blupspin "github.com/ernestrc/blue/upspin"
	multierr "github.com/ernestrc/go-multierror"
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
	configKeys = []string{"username", "keyserver",
		"dirserver", "storeserver", "packing", "secrets", "tlscerts"}
	defaultWorkers int
)

func init() {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	defaultWorkers = int(math.Max(1, math.Min(float64(maxProcs), float64(numCPU))))
}

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

func (s *scheme) init(config config.Config, uri workspace.URI) error {
	cfg, err := initUpspinConfig(config)
	if err != nil {
		return err
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
	return workspace.WorkspaceURI(s.uri, path)
}

func (s *scheme) Open(path string, flag int, mode os.FileMode) (workspace.File, *workspace.Error) {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return nil, workspace.NopError(err)
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
	if flag&os.O_CREATE != 0 {
		// make sure entry is created
		err := f.Sync()
		if err != nil {
			return nil, mapUpspinError(err)
		}
	}
	path, _ = s.expandPath(path)
	return fileAdapter{client: s.client, file: f, path: path}, nil
}

func (s *scheme) Remove(path string) error {
	uname, err := s.makeUpspinPathname(path)
	if err != nil {
		return err
	}
	err = s.client.Delete(uname)
	if err != nil {
		return err
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
		return err
	}

	_, err = s.client.Rename(uold, unew)
	if err != nil {
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
		return nil, err
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
		return nil, err
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
		return "", err
	}
	if entry.Link == "" {
		return "", errors.E("not a link")
	}
	return string(entry.Link), nil
}

type listFilesIterator struct {
	mu  sync.Mutex
	err error
	ctx context.Context
	ch  chan string
}

func (l *listFilesIterator) Next() (string, bool) {
	select {
	case <-l.ctx.Done():
		l.mu.Lock()
		defer l.mu.Unlock()
		l.err = multierr.Append(l.err, l.ctx.Err())
		return "", false
	case path, ok := <-l.ch:
		return path, ok
	}
}

func (l *listFilesIterator) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.err == nil {
		return l.ctx.Err()
	}
	if l.ctx.Err() == nil {
		return l.err
	}
	return multierr.Append(l.err, l.ctx.Err())
}

func traverseDirectory(
	ctx context.Context, cwd workspace.URI, client upspin.Client, path upspin.PathName,
	wg *sync.WaitGroup, ch chan string, workerCh chan upspin.PathName,
) error {
	defer wg.Done()

	entries, err := client.Glob(string(path))
	if err != nil {
		return fmt.Errorf("upspin.Client.Glob(%s): %s", string(path), err)
	}

	var ret error
	for _, entry := range entries {
		if !entry.IsDir() {
			u, err := url.Parse("upspin://" + string(entry.Name))
			if err != nil {
				ret = multierr.Append(ret, err)
				continue
			}
			filename, _ := filepath.Rel(cwd.Path(),
				filepath.Join(cwd.Path(), filepath.Base(u.Path)))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- filename:
				continue
			}
		}

		uname := upspin.PathName(fmt.Sprintf("%s/*", entry.Name))
		wg.Add(1)
		select {
		case <-ctx.Done():
			wg.Done()
			return ctx.Err()
		case workerCh <- uname:
		default:
			// the rest of workers are busy, keep going
			err := traverseDirectory(ctx, cwd, client, uname, wg, ch, workerCh)
			if err != nil {
				ret = multierr.Append(ret, err)
			}
		}
	}
	return ret
}

func traverseDirWorker(
	ctx context.Context, cwd workspace.URI,
	client upspin.Client, wg *sync.WaitGroup,
	ch chan string, workerCh chan upspin.PathName,
	mu *sync.Mutex, err *error,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case path := <-workerCh:
			dirErr := traverseDirectory(ctx, cwd, client, path, wg, ch, workerCh)
			if dirErr != nil {
				mu.Lock()
				*err = multierr.Append(*err, dirErr)
				mu.Unlock()
			}
		}
	}
}

func (s *scheme) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	uname, err := s.makeUpspinPathname("*")
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	ch := make(chan string)
	workerCh := make(chan upspin.PathName)
	errors := make([]error, defaultWorkers)
	iterator := &listFilesIterator{ctx: ctx, ch: ch}

	for i := 0; i < defaultWorkers; i++ {
		go traverseDirWorker(ctx, s.uri, s.client, &wg, ch, workerCh,
			&iterator.mu, &errors[i])
	}

	wg.Add(1)
	workerCh <- uname

	go func() {
		wg.Wait()
		iterator.mu.Lock()
		defer iterator.mu.Unlock()
		for _, err := range errors {
			if err != nil {
				iterator.err = multierr.Append(iterator.err, err)
			}
		}
		close(ch)
	}()

	return iterator, nil
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

func (s *scheme) NewPty() (workspace.Pty, error) {
	return workspace.Pty{}, errExecute
}

func (s *scheme) SetPtySize(workspace.Pty, int, int) error {
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
