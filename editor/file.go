package editor

//go:generate mockgen -destination=./file_gomock.go -package editor -self_package editor -source file.go

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

const defaultFileMode os.FileMode = 0644

// OsFile is used to abstract *os.File.
type OsFile interface {
	Name() string
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error
	WriteString(str string) (int, error)

	io.Seeker
	io.Reader
	io.Closer
	io.Writer
}

type openFunc func(name string, flag int, perm os.FileMode) (OsFile, error)
type removeFunc func(name string) error
type renameFunc func(oldpath, newpath string) error
type statFunc func(name string) (os.FileInfo, error)

// FileBuffer is a struture which persists all updates to a swap file
// and exposes methods to effectively fsync the contents to disk.
type FileBuffer struct {
	openFunc        openFunc
	removeFunc      removeFunc
	renameFunc      renameFunc
	statFunc        statFunc
	lstatFunc       statFunc
	swapDir         string
	swapFileName    string
	fileName        string
	readOnly        bool
	infoModTime     time.Time
	swapInfoModTime time.Time
	orig, swap      OsFile
	reader          cell.Reader
	delayedError    error
	unflushed       bool
}

// FileBuffer cell.Writer API should not be used publicly
type fileBuf FileBuffer

func swapFileName(swapDir, filePath string) (string, string) {
	if swapDir == "" {
		swapDir = filepath.Dir(filePath)
	}
	return swapDir, path.Join(swapDir, fmt.Sprintf(".%s.swp", filepath.Base(filePath)))
}

func (f *FileBuffer) initSwap(swapDir string, orig OsFile, origPerms os.FileMode) (OsFile, error) {
	swap, err := f.openFunc(f.swapFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, origPerms)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrFileAlreadyOpen
		}
		return nil, err
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

func validateFileType(file OsFile) (os.FileInfo, error) {
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

func (f *FileBuffer) openFile(filePath string, flag int) (
	file OsFile, fileInfo os.FileInfo, err error,
) {
	file, err = f.openFunc(filePath, flag, 0000)
	if err != nil {
		file = nil
		return
	}

	fileInfo, err = validateFileType(file)

	return
}

func (f *FileBuffer) initFiles(filePath, swapDir string, readOnly bool) error {
	flag := os.O_RDWR
	if readOnly {
		flag = os.O_RDONLY
	}
	file, fileInfo, err := f.openFile(filePath, flag)
	if err != nil && os.IsNotExist(err) && !readOnly {
		// delegate opening file to Flush
		err = nil
	}
	if err != nil && os.IsPermission(err) {
		// delegate write error to Flush
		file, fileInfo, err = f.openFile(filePath, os.O_RDONLY)
		if err != nil {
			return err
		}
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

func (f *FileBuffer) initBuffer(buf *cell.Buffer, file OsFile) (err error) {
	buf.Reset()
	reader := newUnixFileReader(buf.Reader())
	buf.WithReader(reader)

	// file could be not created yet
	if file != nil {
		_, err = buf.ReadFrom(file)
		if err != nil {
			return
		}

		defer file.Seek(0, 0)
	}

	if !reader.endsWithEOL() {
		buf.WriteString("\n")
	}

	buf.Subscribe((*fileBuf)(f))

	f.reader = buf

	return nil
}

func (f *FileBuffer) recoverFile(filePath, swapFilePath string, buf *cell.Buffer) error {
	var err error
	f.orig, _, err = f.openFile(filePath, os.O_RDWR)
	if err != nil && os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return err
	}
	swap, swapFileInfo, err := f.openFile(swapFilePath, os.O_RDWR)
	if err != nil {
		return err
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

func osOpenFileFunc() openFunc {
	return func(path string, flag int, perm os.FileMode) (OsFile, error) {
		return os.OpenFile(path, flag, perm)
	}
}

func newOsFileBuffer() *FileBuffer {
	ret := new(FileBuffer)
	ret.openFunc = osOpenFileFunc()
	ret.removeFunc = os.Remove
	ret.renameFunc = os.Rename
	ret.statFunc = os.Stat
	ret.lstatFunc = os.Lstat
	return ret
}

// RecoverFileBuffer recovers the file at filePath with the swap file swapFilePath.
func RecoverFileBuffer(filePath, swapFilePath string, buf *cell.Buffer) (
	*FileBuffer, error,
) {
	ret := newOsFileBuffer()

	err := ret.recoverFile(filePath, swapFilePath, buf)
	if err != nil {
		return nil, err
	}
	return ret, err
}

// Init instantiates opens the file at filePath and initializes
// buf with the contents of it. If swapDir is "", then filePath directory is
// used as a swap directory
func (f *FileBuffer) Init(
	filePath string, buf *cell.Buffer, swapDir string, readOnly bool,
) error {
	err := f.initFiles(filePath, swapDir, readOnly)
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

// NewFileBuffer allocates store for a new FileBuffer and then calls Init.
func NewFileBuffer(
	filePath string, buf *cell.Buffer, swapDir string, readOnly bool,
) (
	*FileBuffer, error,
) {
	ret := newOsFileBuffer()

	err := ret.Init(filePath, buf, swapDir, readOnly)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (f *fileBuf) delayCopySwapError(err error) {
	f.delayedError = fmt.Errorf("Swap file error %s: %s", f.swap.Name(), err)
}

func (f *fileBuf) copyFlushSwapFile() (ok bool) {
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
	_, err = f.swap.WriteString(str)
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

func (f *fileBuf) OnWillUpdate(start, end term.Coordinates, str string) {
}

func (f *fileBuf) OnDidUpdate(from, to term.Coordinates, old string) {
	if f.swap == nil {
		return
	}
	f.copyFlushSwapFile()
	return
}

func (f *FileBuffer) moveFile(sourcePath, destPath string) error {
	err := f.renameFunc(sourcePath, destPath)
	if err != nil {
		return err
	}
	return nil
}

func (f *FileBuffer) touchFile() (err error) {
	f.orig, err = f.openFunc(f.fileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, defaultFileMode)
	if err != nil {
		// file was not created when instantiating this FileBuffer, but now
		// file seems to be there so FileBuffer must be stale.
		if os.IsExist(err) {
			err = ErrStaleData
		}
		return
	}
	return
}

// Flushed returns true if the contents of the Buffer have been flushed to file system.
func (f *FileBuffer) Flushed() bool {
	return !f.unflushed
}

// Flush saves the contents of the buffer to disk. If file was modified by some
// other process, this method returns ErrStaleData.
func (f *FileBuffer) Flush() error {
	if f.swap == nil {
		return ErrFileIsNotWritable
	}

	err := f.delayedError
	if err != nil {
		f.delayedError = nil
		if !(*fileBuf)(f).copyFlushSwapFile() {
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
			origTarget, err = os.Readlink(origTarget)
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
func (f *FileBuffer) Close() error {
	if f.fileName == "" {
		return errors.New("trying to Close an uninitialized FileBuffer")
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
