package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/util"
)

const (
	defaultFileMode os.FileMode = 0644
	fileScheme                  = "file"
)

// osFile is used to abstract *os.File.
type osFile interface {
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

type openFunc func(name string, flag int, perm os.FileMode) (osFile, error)
type removeFunc func(name string) error
type renameFunc func(oldpath, newpath string) error
type statFunc func(name string) (os.FileInfo, error)

// localFile is a struture which persists all updates to a swap file
// and exposes methods to effectively fsync the contents to disk.
type localFile struct {
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
	orig, swap      osFile
	reader          cell.View
	delayedError    error
	unflushed       bool
}

// localFile cell.Editor API should not be used publicly
type fileBuf localFile

func swapFileName(swapDir, filePath string) (string, string) {
	if swapDir == "" {
		swapDir = filepath.Dir(filePath)
	}
	return swapDir, path.Join(swapDir, fmt.Sprintf(".%s.swp", filepath.Base(filePath)))
}

func (f *localFile) initSwap(swapDir string, orig osFile, origPerms os.FileMode) (osFile, error) {
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

func (f *localFile) openFile(filePath string, flag int) (
	file osFile, fileInfo os.FileInfo, err error,
) {
	file, err = f.openFunc(filePath, flag, 0000)
	if err != nil {
		file = nil
		return
	}

	fileInfo, err = validateFileType(file)

	return
}

func (f *localFile) initFiles(filePath, swapDir string, readOnly bool) error {
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

func (f *localFile) initBuffer(buf *cell.Buffer, file osFile) (err error) {
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

	buf.Subscribe((*fileBuf)(f))
	buf.WithView(view)

	f.reader = buf

	return nil
}

func (f *localFile) recoverFile(filePath, swapFilePath string, buf *cell.Buffer) error {
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
	return func(path string, flag int, perm os.FileMode) (osFile, error) {
		return os.OpenFile(path, flag, perm)
	}
}

func newOsLocalFile() *localFile {
	ret := new(localFile)
	ret.openFunc = osOpenFileFunc()
	ret.removeFunc = os.Remove
	ret.renameFunc = os.Rename
	ret.statFunc = os.Stat
	ret.lstatFunc = os.Lstat
	return ret
}

// LocalPath attempts to return a unix path in the local system
// or returns an error if URI could not be mapped to path.
// This function returns an error if URI is empty.
func LocalPath(u URI) (string, error) {
	if u == (URI{}) {
		return "", errors.New("empty URI")
	}
	return localPath(u)
}

func localPath(u URI) (string, error) {
	url, err := url.ParseRequestURI(u.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %s: %s", u.String(), err)
	}
	if url.Scheme != fileScheme {
		return "", fmt.Errorf("non supported scheme: %s", url.Scheme)
	}
	return url.Path, nil
}

// recoverlocalFile recovers the file at filePath with the swap file swapFilePath.
func recoverLocalFile(file, swapFile URI, buf *cell.Buffer) (
	FlusherCloser, error,
) {
	ret := newOsLocalFile()
	filePath, err := localPath(file)
	if err != nil {
		return nil, err
	}
	swapFilePath, err := localPath(swapFile)
	if err != nil {
		return nil, err
	}

	err = ret.recoverFile(filePath, swapFilePath, buf)
	if err != nil {
		return nil, err
	}
	return ret, err
}

// Init instantiates opens the file at filePath and initializes
// buf with the contents of it. If swapDir is "", then filePath directory is
// used as a swap directory
func (f *localFile) init(
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

// openLocalFile allocates store for a new localFile and then calls Init.
func openLocalFile(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (
	FlusherCloser, error,
) {
	ret := newOsLocalFile()
	filePath, err := localPath(file)
	if err != nil {
		return nil, err
	}
	swapDirPath, err := localPath(swapDir)
	if err != nil {
		return nil, err
	}

	err = ret.init(filePath, buf, swapDirPath, readOnly)
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

func (f *fileBuf) OnWillEdit(start, end term.Coordinates, str string) {
}

func (f *fileBuf) OnDidEdit(from, to term.Coordinates, old string) {
	if f.swap == nil {
		return
	}
	f.copyFlushSwapFile()
	return
}

func (f *localFile) moveFile(sourcePath, destPath string) error {
	err := f.renameFunc(sourcePath, destPath)
	if err != nil {
		return err
	}
	return nil
}

func (f *localFile) touchFile() (err error) {
	f.orig, err = f.openFunc(f.fileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, defaultFileMode)
	if err != nil {
		// file was not created when instantiating this localFile, but now
		// file seems to be there so localFile must be stale.
		if os.IsExist(err) {
			err = ErrStaleData
		}
		return
	}
	return
}

// Flushed returns true if the contents of the Buffer have been flushed to file system.
func (f *localFile) Flushed() bool {
	return !f.unflushed
}

// Flush saves the contents of the buffer to disk. If file was modified by some
// other process, this method returns ErrStaleData.
func (f *localFile) Flush() error {
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
func (f *localFile) Close() error {
	if f.fileName == "" {
		return errors.New("trying to Close an uninitialized localFile")
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

// LocalURI returns a URI that references the file at local path.
func LocalURI(path string) (URI, error) {
	absPath, err := extractAbsPath(path)
	if err != nil {
		return URI{}, err
	}

	return URI{
		uri:  "file://" + absPath,
		name: filepath.Base(absPath),
	}, nil
}

func extractAbsPath(filename string) (string, error) {
	usr, _ := user.Current()
	dir := usr.HomeDir
	if filename == "~" {
		filename = dir
	} else if strings.HasPrefix(filename, "~/") {
		filename = filepath.Join(dir, filename[2:])
	}
	return filepath.Abs(filename)
}

func sanitizeFilePath(resource string) string {
	resolvedPath, err := filepath.EvalSymlinks(resource)
	if err != nil {
		resolvedPath = filepath.Clean(resource)
	}
	return util.SanitizeLine(resolvedPath)
}

// DefaultLocalSwapFile returns a file's default swap directory in the
// local file system.
func DefaultLocalSwapFile(swapDir URI, file URI) (URI, error) {
	filePath, err := LocalPath(file)
	if err != nil {
		return URI{}, err
	}
	swapDirPath, err := LocalPath(swapDir)
	if err != nil {
		return URI{}, err
	}
	_, swapFilePath := swapFileName(swapDirPath, filePath)
	return LocalURI(swapFilePath)
}

// DefaultLocalSwapFile returns a file's default swap directory in the
// local file system.
func DefaultLocalSwapDirectory(file URI) (URI, error) {
	filePath, err := LocalPath(file)
	if err != nil {
		return URI{}, err
	}
	_, swapFilePath := swapFileName(filepath.Dir(filePath), filePath)
	return LocalURI(filepath.Dir(swapFilePath))
}
