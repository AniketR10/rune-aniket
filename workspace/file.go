package workspace

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

const (
	defaultFileMode os.FileMode = 0644
)

// file implements the sync (swap file) logic
type file struct {
	// used by Flush, Close and worker only
	// since async work goroutine only starts after
	// file has been fully initialized
	wg      sync.WaitGroup
	ch      chan struct{}
	mu      sync.Mutex
	content string
	scheme  Scheme

	buf             *cell.Buffer
	swapDir         string
	swapFileName    string
	fileName        string
	readOnly        bool
	infoModTime     time.Time
	swapInfoModTime time.Time
	orig, swap      File
	delayedError    error
	unflushed       bool
}

func newFile(p Scheme, path string, buf *cell.Buffer, swapDir string, readOnly bool) (
	*file, error,
) {
	ret := new(file)
	ret.scheme = p

	err := ret.init(path, buf, swapDir, readOnly)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func newFileRecover(p Scheme, path, swapFilePath string, buf *cell.Buffer, force bool) (
	*file, error,
) {
	ret := new(file)
	ret.scheme = p

	err := ret.initRecover(path, swapFilePath, buf, force)
	if err != nil {
		return nil, err
	}
	return ret, err
}

func swapFileName(swapDir, filePath string) (string, string) {
	if swapDir == "" {
		swapDir = filepath.Dir(filePath)
	}
	return swapDir, path.Join(swapDir, fmt.Sprintf(".%s.swp", filepath.Base(filePath)))
}

func (f *file) initSwap(swapDir string, orig File, origPerms os.FileMode) (File, error) {
	swap, osErr := f.scheme.Open(f.swapFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, origPerms)
	if osErr != nil {
		if osErr.IsExist {
			return nil, ErrFileAlreadyOpen
		}
		return nil, osErr.ToError()
	}

	if orig == nil {
		return swap, nil
	}

	content, err := ioutil.ReadAll(orig)
	if err != nil {
		return nil, err
	}
	defer orig.Seek(0, 0)

	_, err = swap.Write(content)
	if err != nil {
		return nil, err
	}

	if err = swap.Sync(); err != nil {
		return nil, err
	}

	return swap, nil
}

func validateFileType(file File) (os.FileInfo, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if fileInfo.IsDir() {
		return nil, ErrFileIsNotRegular
	}

	mode := fileInfo.Mode()
	if mode.IsRegular() || mode&os.ModeSymlink != 0 {
		return fileInfo, nil
	}

	return nil, ErrFileIsNotRegular
}

func (f *file) openFile(filePath string, flag int) (
	File, os.FileInfo, error,
) {
	file, err := f.scheme.Open(filePath, flag, 0000)
	if err != nil {
		return nil, nil, err.ToError()
	}

	fileInfo, verr := validateFileType(file)
	if verr != nil {
		return nil, nil, verr
	}

	return file, fileInfo, nil
}

func (f *file) initFiles(filePath, swapDir string, readOnly bool) error {
	flag := os.O_RDWR
	if readOnly {
		flag = os.O_RDONLY
	}
	file, fileInfo, err := f.openFile(filePath, flag)
	switch err {
	case os.ErrNotExist:
		// create unless read-only mode
		if !readOnly {
			err = nil
		}
	case os.ErrPermission:
		// delegate write error to Flush
		file, fileInfo, err = f.openFile(filePath, os.O_RDONLY)
		readOnly = true
	}
	if err != nil {
		return err
	}

	if !readOnly {
		swapDir, swapFileName := swapFileName(swapDir, filePath)
		if f.swapFileName == "" {
			f.swapFileName = swapFileName
		}

		mode := os.FileMode(defaultFileMode)
		if fileInfo != nil {
			mode = fileInfo.Mode()
		}
		swap, err := f.initSwap(swapDir, file, mode)
		if err != nil {
			return err
		}
		// store swapInfo so we can check update times at Flush
		swapInfo, err := f.scheme.Stat(swap.Name())
		if err != nil {
			return err
		}
		f.swap = swap
		f.swapInfoModTime = swapInfo.ModTime()
		f.swapDir = swapDir
	}

	f.orig = file
	if fileInfo != nil {
		f.infoModTime = fileInfo.ModTime()
	}
	f.fileName = filePath
	f.readOnly = readOnly

	return nil
}

func (f *file) initBuffer(buf *cell.Buffer, file File) (err error) {
	buf.Reset()
	view := newUnixFileReader(buf.View())

	// file could be not created yet
	if file != nil {
		_, err = buf.ReadFrom(file)
		if err != nil {
			return
		}

		defer file.Seek(0, 0)
	}

	if !view.endsWithEOL() {
		buf.WriteString("\n")
	}

	buf.Subscribe(f)
	buf.WithView(view)

	f.buf = buf

	return nil
}

func (f *file) initRecover(filePath, swapFilePath string, buf *cell.Buffer, force bool) error {
	orig, info, err := f.openFile(filePath, os.O_RDWR)
	if err == os.ErrNotExist {
		err = nil
	}
	if err != nil {
		return err
	}
	swap, swapFileInfo, osErr := f.openFile(swapFilePath, os.O_RDWR)
	if osErr != nil {
		return osErr
	}

	f.orig = orig
	f.swap = swap
	if info != nil {
		f.infoModTime = info.ModTime()
	}
	f.swapInfoModTime = swapFileInfo.ModTime()
	f.swapFileName = swapFilePath
	f.swapDir = filepath.Dir(swapFilePath)
	f.fileName = filePath

	err = f.initBuffer(buf, f.swap)
	if err != nil {
		// do not remove swap if error is that swap is out of date
		// let use decide what to do with it
		if clerr := f.swap.Close(); clerr != nil {
			err = multierr.Append(err, clerr)
		}
		f.swap = nil
		if clerr := f.Close(); clerr != nil {
			err = multierr.Append(err, clerr)
		}
		return err
	}

	err = f.flush(force)
	if err != nil {
		buf.Reset()
		if clerr := f.swap.Close(); clerr != nil {
			err = multierr.Append(err, clerr)
		}
		f.swap = nil
		if clerr := f.Close(); clerr != nil {
			err = multierr.Append(err, clerr)
		}
		return err
	}

	f.setupCopySwapWorker()

	return nil
}

func (f *file) setupCopySwapWorker() {
	// a buffered channel of 1 guarantees that if worker
	// is busy and the call to copyFlushSwap is skipped
	// we are going to copyFlushSwap at least one final time
	f.ch = make(chan struct{}, 1)

	go func(ch chan struct{}) {
		for range ch {
			f.mu.Lock()
			str := f.content
			f.mu.Unlock()
			f.copyFlushSwapFile(str)
			f.wg.Done()
		}
	}(f.ch)
}

// init instantiates opens the file at filePath and initializes
// buf with the contents of it. If swapDir is "", then filePath directory is
// used as a swap directory
func (f *file) init(
	file string, buf *cell.Buffer, swapDir string, readOnly bool,
) error {
	err := f.initFiles(file, swapDir, readOnly)
	if err != nil {
		return err
	}

	err = f.initBuffer(buf, f.orig)
	if err != nil {
		if clerr := f.Close(); clerr != nil {
			err = multierr.Append(err, clerr)
		}
		return err
	}

	f.setupCopySwapWorker()

	return nil
}

func (f *file) delayCopySwapError(err error) {
	f.delayedError = fmt.Errorf("Swap file error %s: %s", f.swap.Name(), err)
}

func (f *file) copyFlushSwapFile(str string) (ok bool) {
	if f.swap == nil {
		return
	}

	f.unflushed = true
	err := f.swap.Truncate(0)
	if err != nil {
		f.delayCopySwapError(err)
		return
	}
	_, err = f.swap.Seek(0, 0)
	if err != nil {
		f.delayCopySwapError(err)
		return
	}

	// files must end in EOL
	if !strings.HasSuffix(str, "\n") {
		str += "\n"
	}
	_, err = f.swap.Write([]byte(str))
	if err != nil {
		f.delayCopySwapError(err)
		return
	}

	err = f.swap.Sync()
	if err != nil {
		f.delayCopySwapError(err)
		return
	}

	f.swapInfoModTime = time.Now()
	ok = true
	return
}

func (f *file) OnWillEdit(start, end term.Coordinates, str string) {
	f.wg.Add(1)
}

func (f *file) OnDidEdit(from, to term.Coordinates, old string) {
	// store the latest version of the buffer so the last
	// copyFlushSwap to run uses the up-to-date version.
	f.mu.Lock()
	f.content = f.buf.String()
	f.mu.Unlock()
	select {
	case f.ch <- struct{}{}:
	default:
		f.wg.Done()
	}
}

func (f *file) moveFile(sourcePath, destPath string) error {
	err := f.scheme.Rename(sourcePath, destPath)
	if err != nil {
		return err
	}
	return nil
}

func (f *file) touchFile() error {
	var err *Error
	f.orig, err = f.scheme.Open(f.fileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, defaultFileMode)
	if err != nil {
		// file was not created when instantiating this file, but now
		// file seems to be there so file must be stale.
		if err.IsExist {
			return ErrStaleData
		}
	}
	return nil
}

// Flush saves the contents of the buffer to disk. If file was modified by some
// other process, this method returns ErrStaleData. Flush blocks until
// all edits have been processed.
func (f *file) Flush() error {
	return f.flush(false)
}

func (f *file) flush(force bool) error {
	f.wg.Wait()

	if f.swap == nil {
		return ErrFileIsNotWritable
	}

	err := f.delayedError
	if err != nil {
		f.delayedError = nil
		if !f.copyFlushSwapFile(f.buf.String()) {
			return err
		}
	}

	// used to override with symlink target if applicable
	origTarget := f.fileName

	// create file if it didn't exist before
	if f.orig == nil {
		err := f.touchFile()
		if err != nil {
			return err
		}
	} else {
		newFileInfo, err := f.scheme.Stat(f.orig.Name())
		if err != nil {
			return err
		}
		if !force && (newFileInfo.ModTime().After(f.swapInfoModTime) ||
			newFileInfo.ModTime().After(f.infoModTime)) {
			return ErrStaleData
		}

		newFileInfo, err = f.scheme.Lstat(f.orig.Name())
		if err != nil {
			return err
		}
		if newFileInfo.Mode()&os.ModeSymlink != 0 {
			origTarget, err = f.scheme.ReadLink(origTarget)
			if err != nil {
				return err
			}
		}
	}

	newSwapInfo, err := f.scheme.Stat(f.swap.Name())
	if err != nil {
		return err
	}
	if !force && newSwapInfo.ModTime().After(f.swapInfoModTime) {
		return ErrStaleData
	}

	err = f.moveFile(f.swap.Name(), origTarget)
	if err != nil {
		return err
	}

	// any fs errors should be picked up or fixed
	// by opening files again
	_ = f.orig.Close()
	_ = f.swap.Close()

	err = f.initFiles(f.fileName, f.swapDir, f.readOnly)
	if err != nil {
		return err
	}

	f.unflushed = false

	return nil
}

// Close should be called once when this structure is not to be used anymore.
func (f *file) Close() (ret error) {
	if f.fileName == "" {
		return errors.New("trying to Close an uninitialized file")
	}

	f.fileName = ""

	// Wait for all async work to complete.
	// Calling goroutine should be the same goroutine
	// that calls OnDidEdit so no more work should be added
	f.wg.Wait()

	if f.ch != nil {
		close(f.ch)
	}

	if f.buf != nil {
		f.buf.Unsubscribe(f)
	}

	if f.swap != nil {
		swapFileName := f.swap.Name()
		if err := f.swap.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
		newSwapInfo, err := f.scheme.Stat(f.swap.Name())
		if err != nil {
			ret = multierr.Append(ret, err)
		} else {
			// do not remove a swap from another process
			if !newSwapInfo.ModTime().After(f.swapInfoModTime) {
				if err := f.scheme.Remove(swapFileName); err != nil {
					ret = multierr.Append(ret, err)
				}
			}
		}
		f.swap = nil
	}

	if f.orig != nil {
		if err := f.orig.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
		f.orig = nil
	}

	return ret
}
