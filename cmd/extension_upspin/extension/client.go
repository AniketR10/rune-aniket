package extension

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ernestrc/blue/retry"
	blupspin "github.com/ernestrc/blue/upspin"
	"upspin.io/errors"
	"upspin.io/upspin"
)

var (
	retryTimeout  = 5 * time.Second
	retryStrategy = retry.CombinedStrategy(
		retry.LimitStrategy(10),
		retry.ExponentialStrategy(10*time.Millisecond, 1000*time.Millisecond),
	)
)

// consistent upspin.Client which overrides key methods
// to store last updated sequence ID and retry lookups
// until last sequence ID matches.
type upspinClient struct {
	upspin.Client
	lastSequenceID sync.Map // map[upspin.PathName]int64
}

func newUpspinClient(client upspin.Client) *upspinClient {
	return &upspinClient{
		Client: client,
	}
}

func (c *upspinClient) Lookup(
	name upspin.PathName, followFinal bool,
) (*upspin.DirEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()

	var ret *upspin.DirEntry
	err := retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		var err error
		ret, err = c.Client.Lookup(name, followFinal)
		if err != nil {
			return !errors.Is(errors.NotExist, err), err
		}
		lastSeqID, ok := c.lastSequenceID.Load(name)
		if ok && ret.Sequence < lastSeqID.(int64) {
			return true, errors.E("stale Lookup")
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *upspinClient) Rename(oldName, newName upspin.PathName) (
	*upspin.DirEntry, error,
) {
	entry, err := c.Client.Rename(oldName, newName)
	if err != nil {
		return nil, err
	}

	c.lastSequenceID.Store(entry.Name, entry.Sequence)
	c.lastSequenceID.Delete(oldName)

	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()

	err = retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		_, err = c.Client.Lookup(oldName, false)
		if err == nil {
			return true, errors.E("stale lookup")
		}
		if errors.Is(errors.NotExist, err) {
			return false, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	err = retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		_, err = c.Client.Lookup(newName, false)
		if err == nil {
			return false, nil
		}
		if errors.Is(errors.NotExist, err) {
			return true, errors.E("stale lookup")
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return entry, err
}

func (c *upspinClient) Delete(name upspin.PathName) error {
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()

	err := c.Client.Delete(name)
	if err != nil {
		return err
	}

	c.lastSequenceID.Delete(name)

	err = retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		_, err = c.Client.Lookup(name, false)
		if err == nil {
			return true, errors.E("stale lookup")
		}
		if errors.Is(errors.NotExist, err) {
			return false, nil
		}
		return false, err
	})
	return err
}

func (c *upspinClient) Put(name upspin.PathName, data []byte) (
	*upspin.DirEntry, error,
) {
	entry, err := c.Client.Put(name, data)
	if err == nil {
		c.lastSequenceID.Store(entry.Name, entry.Sequence)
	}
	return entry, err
}

// satisfies internal osFile (only diff with upspin.File is Name())
type fileAdapter struct {
	s         *scheme
	path      string
	lastEntry *upspin.DirEntry
	client    *upspinClient
	file      *blupspin.File
	fd        uintptr
}

// satisfies os.FileInfo
type entryAdapter struct {
	entry *upspin.DirEntry
}

func (e entryAdapter) Name() string {
	// based on os.FileInfo, Name always returns the base
	// name of the file.
	return filepath.Base(string(e.entry.Name))
}

func (f fileAdapter) Fd() uintptr {
	return f.fd
}

func (e entryAdapter) Size() (ret int64) {
	// unused, but still need to return something
	// so rpc server does not panic
	return 0
}

func (e entryAdapter) Mode() fs.FileMode {
	// only used for symlink
	if e.entry.Attr == upspin.AttrLink {
		return fs.ModeSymlink
	}
	return 0
}

func (e entryAdapter) ModTime() time.Time {
	return time.Unix(int64(e.entry.Time), 0)
}

func (e entryAdapter) IsDir() bool {
	return e.entry.IsDir()
}

func (e entryAdapter) Sys() interface{} {
	return nil
}

func (f *fileAdapter) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

func (f *fileAdapter) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

func (f *fileAdapter) Close() error {
	// do not call upspin file.Close or
	// we will issue a Sync, which breaks
	// workspaceapi.File semantics.
	delete(f.s.files, f.Fd())
	return nil
}

func (f *fileAdapter) Write(p []byte) (n int, err error) {
	n, err = f.file.Write(p)
	if err != nil {
		return
	}
	return
}

func (f *fileAdapter) Name() string {
	// Name as defined by os.File is always whatever
	// path was given to Open
	return f.path
}

func (f *fileAdapter) Stat() (os.FileInfo, error) {
	return entryAdapter{entry: f.lastEntry}, nil
}

func (f *fileAdapter) Sync() error {
	entry, err := f.file.Sync()
	if err != nil {
		return err
	}
	f.lastEntry = entry
	return nil
}

func (f *fileAdapter) Truncate(size int64) error {
	return f.file.Truncate(int(size))
}
