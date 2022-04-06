package workspace

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

const (
	defaultFileMode os.FileMode = 0644
	fileScheme                  = "file"
)

// file is a struture which persists all updates to a swap file
// and exposes methods to effectively fsync the contents to disk.
type file struct {
	openFunc        openFunc
	removeFunc      removeFunc
	renameFunc      renameFunc
	statFunc        statFunc
	lstatFunc       statFunc
	readLinkFunc    readLinkFunc
	swapDir         string
	swapFileName    string
	fileName        string
	readOnly        bool
	infoModTime     time.Time
	swapInfoModTime time.Time
	orig, swap      osFile
	reader          cell.View
	delayedError    error
	unflushed       bool
}

func swapFileName(swapDir, filePath string) (string, string) {
	if swapDir == "" {
		swapDir = filepath.Dir(filePath)
	}
	return swapDir, path.Join(swapDir, fmt.Sprintf(".%s.swp", filepath.Base(filePath)))
}

func (f *file) initSwap(swapDir string, orig osFile, origPerms os.FileMode) (osFile, error) {
	swap, osErr := f.openFunc(f.swapFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, origPerms)
	if osErr != nil {
		if osErr.isExist {
			return nil, ErrFileAlreadyOpen
		}
		return nil, osErr
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
		swap.Close()
		return nil, err
	}

	if err = swap.Sync(); err != nil {
		return nil, err
	}

	return swap, nil
}

func validateFileType(file osFile) (os.FileInfo, error) {
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
	osFile, os.FileInfo, error,
) {
	file, err := f.openFunc(filePath, flag, 0000)
	if err != nil {
		return nil, nil, err
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
	if err != nil {
		if osErr, ok := err.(*osError); ok && osErr.isNotExist && !readOnly {
			err = nil
		}
	}
	if err != nil {
		// delegate write error to Flush
		if osErr, ok := err.(*osError); ok && osErr.isPermission {
			file, fileInfo, err = f.openFile(filePath, os.O_RDONLY)
			if err != nil {
				return err
			}
			readOnly = true
		}
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
		swapInfo, err := f.statFunc(swap.Name())
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

func (f *file) initBuffer(buf *cell.Buffer, file osFile) (err error) {
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

	f.reader = buf

	return nil
}

func (f *file) recoverFile(filePath, swapFilePath string, buf *cell.Buffer) error {
	var err error
	f.orig, _, err = f.openFile(filePath, os.O_RDWR)
	if err != nil {
		if osErr, ok := err.(*osError); ok && osErr.isNotExist {
			err = nil
		}
	}
	if err != nil {
		return err
	}
	swap, swapFileInfo, osErr := f.openFile(swapFilePath, os.O_RDWR)
	if osErr != nil {
		return osErr
	}

	f.swap = swap
	f.infoModTime = swapFileInfo.ModTime()
	f.swapInfoModTime = swapFileInfo.ModTime()
	f.swapFileName = swapFilePath
	f.swapDir = filepath.Dir(swapFilePath)
	f.fileName = filePath

	err = f.initBuffer(buf, f.swap)
	if err != nil {
		f.Close()
		return err
	}

	err = f.Flush()
	if err != nil {
		buf.Reset()
		f.Close()
		return err
	}

	return nil
}

// Init instantiates opens the file at filePath and initializes
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
		f.Close()
		return err
	}

	return nil
}

func (f *file) delayCopySwapError(err error) {
	f.delayedError = fmt.Errorf("Swap file error %s: %s", f.swap.Name(), err)
}

func (f *file) copyFlushSwapFile() (ok bool) {
	f.unflushed = true
	str := f.reader.String()
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
}

func (f *file) OnDidEdit(from, to term.Coordinates, old string) {
	if f.swap == nil {
		return
	}
	f.copyFlushSwapFile()
	return
}

func (f *file) moveFile(sourcePath, destPath string) error {
	err := f.renameFunc(sourcePath, destPath)
	if err != nil {
		return err
	}
	return nil
}

func (f *file) touchFile() error {
	var err *osError
	f.orig, err = f.openFunc(f.fileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, defaultFileMode)
	if err != nil {
		// file was not created when instantiating this file, but now
		// file seems to be there so file must be stale.
		if err.isExist {
			return ErrStaleData
		}
	}
	return nil
}

// Flushed returns true if the contents of the Buffer have been flushed to file system.
func (f *file) Flushed() bool {
	return !f.unflushed
}

// Flush saves the contents of the buffer to disk. If file was modified by some
// other process, this method returns ErrStaleData.
func (f *file) Flush() error {
	if f.swap == nil {
		return ErrFileIsNotWritable
	}

	err := f.delayedError
	if err != nil {
		f.delayedError = nil
		if !f.copyFlushSwapFile() {
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
		newFileInfo, err := f.statFunc(f.orig.Name())
		if err != nil {
			return err
		}
		if newFileInfo.ModTime().After(f.infoModTime) {
			return ErrStaleData
		}

		newFileInfo, err = f.lstatFunc(f.orig.Name())
		if err != nil {
			return err
		}
		if newFileInfo.Mode()&os.ModeSymlink != 0 {
			origTarget, err = f.readLinkFunc(origTarget)
			if err != nil {
				return err
			}
		}
	}

	newSwapInfo, err := f.statFunc(f.swap.Name())
	if err != nil {
		return err
	}
	if newSwapInfo.ModTime().After(f.swapInfoModTime) {
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
func (f *file) Close() error {
	if f.fileName == "" {
		return errors.New("trying to Close an uninitialized file")
	}

	f.fileName = ""

	var err2, err3 error
	if f.swap != nil {
		swapFileName := f.swap.Name()
		err2 = f.swap.Close()

		newSwapInfo, err := f.statFunc(f.swap.Name())
		if err != nil {
			return err
		}
		if !newSwapInfo.ModTime().After(f.swapInfoModTime) {
			err3 = f.removeFunc(swapFileName)
		}
		f.swap = nil
	}

	if f.orig != nil {
		err := f.orig.Close()
		if err != nil {
			return err
		}
	}

	if err2 != nil {
		return err2
	}
	if err3 != nil {
		return err3
	}

	return nil
}
