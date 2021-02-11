package editor

//go:generate mockgen -destination=./file_gomock.go -package editor -self_package editor -source file.go

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

const filePerms = 0600

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
	openFunc     openFunc
	removeFunc   removeFunc
	renameFunc   renameFunc
	statFunc     statFunc
	swapDir      string
	swapFileName string
	fileName     string
	info         os.FileInfo
	orig, swap   OsFile
	reader       cell.PublisherReader
	delayedError error
	unflushed    bool
}

// FileBuffer cell.Writer API should not be used publicly
type fileBuf FileBuffer

func makeSwapFileName(filename string) string {
	return fmt.Sprintf(".%s.swp", filename)
}

func (f *FileBuffer) initSwap(swapDir string, orig OsFile) (OsFile, error) {
	swap, err := f.openFunc(f.swapFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, filePerms)
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

func (f *FileBuffer) openFile(filePath string) (
	file OsFile, fileInfo os.FileInfo, err error,
) {
	file, err = f.openFunc(filePath, os.O_RDWR, filePerms)
	if err != nil {
		file = nil
		return
	}

	fileInfo, err = validateFileType(file)

	return
}

func (f *FileBuffer) initFiles(filePath, swapDir string) error {
	file, fileInfo, err := f.openFile(filePath)
	if err != nil && os.IsNotExist(err) {
		// delegate opening file to Flush
		err = nil
	}
	if err != nil {
		return err
	}

	if swapDir == "" {
		swapDir = filepath.Dir(filePath)
	}

	if f.swapFileName == "" {
		f.swapFileName = path.Join(swapDir, makeSwapFileName(filepath.Base(filePath)))
	}

	swap, err := f.initSwap(swapDir, file)
	if err != nil {
		return err
	}

	f.orig = file
	f.swap = swap
	f.info = fileInfo
	f.fileName = filePath
	f.swapDir = swapDir

	return nil
}

func (f *FileBuffer) initBuffer(buf *cell.Buffer, file OsFile) (err error) {
	buf.Reset()

	// file could be not created yet
	if file != nil {
		_, err = buf.ReadFrom(file)
		if err != nil {
			return
		}

		defer file.Seek(0, 0)
	}

	f.reader = buf

	buf.Subscribe((*fileBuf)(f))

	return nil
}

func (f *FileBuffer) recoverFile(filePath, swapFilePath string, buf *cell.Buffer) error {
	var err error
	f.orig, _, err = f.openFile(filePath)
	if err != nil && os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return err
	}
	swap, swapFileInfo, err := f.openFile(swapFilePath)
	if err != nil {
		return err
	}

	f.swap = swap
	f.info = swapFileInfo
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
	usr, _ := user.Current()
	dir := usr.HomeDir
	return func(path string, flag int, perm os.FileMode) (OsFile, error) {
		if path == "~" {
			path = dir
		} else if strings.HasPrefix(path, "~/") {
			path = filepath.Join(dir, path[2:])
		}
		return os.OpenFile(path, flag, perm)
	}
}

func newOsFileBuffer() *FileBuffer {
	ret := new(FileBuffer)
	ret.openFunc = osOpenFileFunc()
	ret.removeFunc = os.Remove
	ret.renameFunc = os.Rename
	ret.statFunc = os.Stat
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
func (f *FileBuffer) Init(filePath string, buf *cell.Buffer, swapDir string) error {
	err := f.initFiles(filePath, swapDir)
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
func NewFileBuffer(filePath string, buf *cell.Buffer, swapDir string) (
	*FileBuffer, error,
) {
	ret := newOsFileBuffer()

	err := ret.Init(filePath, buf, swapDir)
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
	_, err = f.swap.WriteString(str)
	if err != nil {
		f.delayCopySwapError(err)
		return
	}

	// files must end in EOL; cell.Buffer hides the last EOL
	// so it is safe here to always write a last EOL.
	_, err = f.swap.Write([]byte{'\n'})
	if err != nil {
		f.delayCopySwapError(err)
		return
	}

	err = f.swap.Sync()
	if err != nil {
		f.delayCopySwapError(err)
		return
	}

	ok = true
	return
}

func (f *fileBuf) OnWillInsert(at term.Coordinates, str string) {
}

func (f *fileBuf) OnDidInsert(from, to term.Coordinates) {
	f.copyFlushSwapFile()
}

func (f *fileBuf) OnWillDelete(from, to term.Coordinates) {
}

func (f *fileBuf) OnDidDelete(start, end term.Coordinates, str string) {
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
	f.orig, err = f.openFunc(f.fileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, filePerms)
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
	err := f.delayedError
	if err != nil {
		f.delayedError = nil
		if !(*fileBuf)(f).copyFlushSwapFile() {
			return err
		}
	}

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
		if newFileInfo.ModTime().After(f.info.ModTime()) {
			return ErrStaleData
		}
	}

	err = f.moveFile(f.swap.Name(), f.orig.Name())
	if err != nil {
		return err
	}

	// any fs errors should be picked up or fixed
	// by opening files again
	_ = f.orig.Close()
	_ = f.swap.Close()

	err = f.initFiles(f.fileName, f.swapDir)
	if err != nil {
		return err
	}

	f.unflushed = false

	return nil
}

// Close should be called once when this structure is not to be used anymore.
func (f *FileBuffer) Close() error {
	if f.swap == nil {
		return errors.New("trying to Close an uninitialized FileBuffer")
	}
	swapFileName := f.swap.Name()
	err2 := f.swap.Close()
	err3 := f.removeFunc(swapFileName)
	f.swap = nil

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
