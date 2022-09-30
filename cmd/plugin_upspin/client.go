package main

import (
	"context"
	"io/fs"
	"os"
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
		retry.ExponentialStrategy(1*time.Millisecond, 1000*time.Millisecond),
	)
)

// consistent upspin.Client which overrides key methods
// to store last updated sequence ID and retry lookups
// until last sequence ID matches.
type upspinClient struct {
	upspin.Client
	lastSequenceID map[upspin.PathName]int64
}

func newUpspinClient(client upspin.Client) *upspinClient {
	return &upspinClient{
		Client:         client,
		lastSequenceID: make(map[upspin.PathName]int64),
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
			return errors.Is(errors.NotExist, err), err
		}
		lastSeqID, ok := c.lastSequenceID[name]
		if ok && ret.Sequence < lastSeqID {
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
	if err == nil {
		c.lastSequenceID[entry.Name] = entry.Sequence
		delete(c.lastSequenceID, oldName)
	}
	return entry, err
}

func (c *upspinClient) Delete(name upspin.PathName) error {
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()

	err := c.Client.Delete(name)
	if err == nil {
		delete(c.lastSequenceID, name)
	}

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
		c.lastSequenceID[entry.Name] = entry.Sequence
	}
	return entry, err
}

// satisfies internal osFile (only diff with upspin.File is Name())
type fileAdapter struct {
	client *upspinClient
	file   *blupspin.File
}

// satisfies os.FileInfo
type entryAdapter struct {
	entry *upspin.DirEntry
}

func (e entryAdapter) Name() string {
	return string(e.entry.Name)
}

func (e entryAdapter) Size() int64 {
	// unused
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
	// allows for same types of checks and we can
	// take advantage of the eventual-consistency provided
	// by the backend systems.
	return time.Unix(int64(e.entry.Sequence), 0)
}

func (e entryAdapter) IsDir() bool {
	return e.entry.IsDir()
}

func (e entryAdapter) Sys() interface{} {
	return nil
}

func (f fileAdapter) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

func (f fileAdapter) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

func (f fileAdapter) Close() error {
	// respect os.File semantics:
	// do not call Put once more to avoid conflicts with rename
	return nil
}

func (f fileAdapter) Write(p []byte) (n int, err error) {
	return f.file.Write(p)
}

func (f fileAdapter) Name() string {
	return string(f.file.Name())
}

func (f fileAdapter) Stat() (os.FileInfo, error) {
	entry, err := f.client.Lookup(f.file.Name(), true)
	if err != nil {
		return nil, err
	}
	return entryAdapter{entry: entry}, nil
}

func (f fileAdapter) Sync() error {
	return f.file.Sync()
}

func (f fileAdapter) Truncate(size int64) error {
	return f.file.Truncate(int(size))
}
