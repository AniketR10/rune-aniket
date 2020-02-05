package editor

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/term"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/* INTEGRATION TESTS */

func newIntegrationTestCase(t *testing.T, endsInEOL bool) (
	*cell.Buffer, *os.File, func(),
) {
	buffer := cell.NewBuffer()
	file, err := ioutil.TempFile("", "frctl_file_test")
	require.NoError(t, err)

	_, err = file.Write([]byte(sampleSnippet))
	require.NoError(t, err)

	if endsInEOL {
		_, err = file.Write([]byte{'\n'})
		require.NoError(t, err)
	}

	return buffer, file, func() {
		file.Close()
		os.Remove(file.Name())
	}
}

// tests FileBuffer with real os.File's. endsInEOL refers to the original file.
func testFileBufferIntegration(t *testing.T, endsInEOL bool) {
	t.Run("if file does not exist, create it upon Flush", func(t *testing.T) {
		buf := cell.NewBuffer()
		file, err := ioutil.TempFile("", "frctl_file_test")
		require.NoError(t, err)

		// secure a random filename in a tmp directory
		filename := file.Name()
		require.NoError(t, os.Remove(filename))

		f, err := NewFileBuffer(filename, buf, "")
		require.NoError(t, err)

		_, err = os.Stat(filename)
		require.Error(t, err)

		require.NoError(t, f.Flush())

		_, err = os.Stat(filename)
		require.NoError(t, err)
	})

	t.Run("if file does not exist, if there are errors upon creation, it bubbles up on Flush", func(t *testing.T) {
		buf := cell.NewBuffer()
		rand.Seed(int64(time.Now().Nanosecond()))
		filename := fmt.Sprintf("/tmp/mpo/tmp/tmp/tmp/tmp/%d.go", rand.Int())

		f, err := NewFileBuffer(filename, buf, os.TempDir())
		require.NoError(t, err)

		_, err = os.Stat(filename)
		require.Error(t, err)

		require.Error(t, f.Flush())
	})

	t.Run("no swap file is open, creates one; removes on close", func(t *testing.T) {
		b, file, cleanup := newIntegrationTestCase(t, endsInEOL)
		defer cleanup()

		swapDir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		swapFileName := path.Join(swapDir, makeSwapFileName(filepath.Base(file.Name())))

		_, err = os.Stat(swapFileName)
		require.Error(t, err)

		f, err := NewFileBuffer(file.Name(), b, swapDir)
		require.NoError(t, err)

		_, err = os.Stat(swapFileName)
		require.NoError(t, err, swapFileName)

		f.Close()

		_, err = os.Stat(swapFileName)
		assert.Error(t, err)
	})

	t.Run("fsyncs swap upon update", func(t *testing.T) {
		b, file, cleanup := newIntegrationTestCase(t, endsInEOL)
		defer cleanup()

		f, err := NewFileBuffer(file.Name(), b, "")
		require.NoError(t, err)

		defer f.Close()

		buf, err := ioutil.ReadFile(f.swap.Name())
		require.NoError(t, err)
		content := sampleSnippet
		if endsInEOL {
			content += "\n"
		}
		assert.Equal(t, content, string(buf))

		const writeStr = "XXXXXX"
		b.InsertString(term.Coordinates{}, writeStr)

		buf, err = ioutil.ReadFile(f.swap.Name())
		require.NoError(t, err)

		assert.Equal(t, writeStr+sampleSnippet+"\n", string(buf))
	})

	t.Run("fsyncs file upon Flush", func(t *testing.T) {
		b, file, cleanup := newIntegrationTestCase(t, endsInEOL)
		defer cleanup()

		f, err := NewFileBuffer(file.Name(), b, "")
		require.NoError(t, err)
		defer f.Close()

		const writeStr = "XXXXXX"
		b.InsertString(term.Coordinates{}, writeStr)
		require.NoError(t, f.Flush())

		buf, err := ioutil.ReadFile(file.Name())
		require.NoError(t, err)

		assert.Equal(t, writeStr+sampleSnippet+"\n", string(buf))
	})

	t.Run("returns error if swap is already open", func(t *testing.T) {
		b, file, cleanup := newIntegrationTestCase(t, endsInEOL)
		defer cleanup()

		swapDir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		f, err := NewFileBuffer(file.Name(), b, swapDir)
		require.NoError(t, err)
		defer f.Close()

		_, err = NewFileBuffer(file.Name(), b, swapDir)
		assert.Equal(t, ErrFileAlreadyOpen, err)
	})
}

func TestFileBufferIntegrationEOL(t *testing.T) {
	testFileBufferIntegration(t, true)
}

func TestFileBufferIntegrationNOEOL(t *testing.T) {
	testFileBufferIntegration(t, false)
}

func assertRecoverFromSwapFile(t *testing.T, filename, swapname string, b *cell.Buffer) {
	assert.Equal(t, sampleSnippet, b.String())

	buf, err := ioutil.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, sampleSnippet+"\n", string(buf))

	buf, err = ioutil.ReadFile(swapname)
	require.NoError(t, err)
	assert.Equal(t, sampleSnippet+"\n", string(buf))
}

func newRecoveryIntegrationCase(t *testing.T) (
	*cell.Buffer, *os.File, *os.File, func(),
) {
	_, file, cleanupFile := newIntegrationTestCase(t, true)
	require.NoError(t, file.Truncate(0))

	b, swap, cleanupSwap := newIntegrationTestCase(t, true)

	return b, file, swap, func() {
		cleanupSwap()
		cleanupFile()
	}
}

func TestFileBufferRecover(t *testing.T) {
	t.Run("recovers file from swap", func(t *testing.T) {
		b, file, swap, cleanup := newRecoveryIntegrationCase(t)
		defer cleanup()

		filepath, swapFilepath := file.Name(), swap.Name()
		f, err := RecoverFile(filepath, swapFilepath, b)
		require.NoError(t, err)
		defer f.Close()

		assertRecoverFromSwapFile(t, filepath, swapFilepath, b)
	})

	t.Run("recovers file from swap even if file does not exist", func(t *testing.T) {
		b, swap, cleanup := newIntegrationTestCase(t, true)
		defer cleanup()

		swapFilepath := swap.Name()
		swapFileName := path.Base(swapFilepath)
		filepath := path.Join(path.Dir(swapFilepath), "my_actual_file"+swapFileName)
		f, err := RecoverFile(filepath, swapFilepath, b)
		require.NoError(t, err, filepath)
		defer f.Close()

		assertRecoverFromSwapFile(t, filepath, swapFilepath, b)
	})

	t.Run("returns error if file was modified after swap", func(t *testing.T) {
		b, file, swap, cleanup := newRecoveryIntegrationCase(t)
		defer cleanup()

		time.Sleep(100 * time.Millisecond)
		file.WriteString("blah")
		file.Sync()

		filepath, swapFilepath := file.Name(), swap.Name()
		_, err := RecoverFile(filepath, swapFilepath, b)
		require.Equal(t, ErrStaleData, err)
	})
}

/* UNIT TESTS */

// implements os.FileInfo
type testFileInfo struct {
	name    string
	isDir   bool
	modTime time.Time
	size    int64
	mode    os.FileMode
}

func (t testFileInfo) Name() string {
	return t.name
}
func (t testFileInfo) Size() int64 {
	return t.size
}

func (t testFileInfo) Mode() os.FileMode {
	return t.mode
}

func (t testFileInfo) ModTime() time.Time {
	return t.modTime
}

func (t testFileInfo) IsDir() bool {
	return t.isDir
}

func (t testFileInfo) Sys() interface{} {
	return nil
}

// returns an un-initialized (but dep injected) FileBuffer along with the mocked osFile
func newTestFileBuffer(ctrl *gomock.Controller) (*FileBuffer, *MockOsFile) {
	f := new(FileBuffer)
	mock := NewMockOsFile(ctrl)
	f.openFunc = func(name string, flag int, perm os.FileMode) (osFile, error) {
		return mock, nil
	}
	f.removeFunc = func(name string) error {
		return nil
	}
	f.renameFunc = func(oldName, newName string) error {
		return nil
	}
	return f, mock
}

func expectRead(mock *MockOsFile, data []byte) {
	mock.EXPECT().Read(gomock.Any()).DoAndReturn(func(buf []byte) (int, error) {
		if len(buf) < len(data) {
			panic("seriously?")
		}
		n := copy(buf, data)
		return n, io.EOF
	})
}

func expectInitSwap(
	mock *MockOsFile, fileName string, fileInfo os.FileInfo, data []byte,
) {
	// stat original file
	mock.EXPECT().Stat().Return(fileInfo, nil)
	// read original file
	expectRead(mock, data)

	mock.EXPECT().Name().Return(fileName).AnyTimes()
	// seek original file back to 0
	mock.EXPECT().Seek(gomock.Eq(int64(0)), gomock.Eq(0)).Return(int64(0), nil)

	// write to swap file
	mock.EXPECT().Write(gomock.Any()).DoAndReturn(func(p []byte) (n int, err error) {
		// assert.EqualValues(t, data, p)
		return len(p), nil
	})

	// sync to swap file
	mock.EXPECT().Sync().Return(nil)
}

func expectInitBuffer(mock *MockOsFile, data []byte) {
	expectRead(mock, data)

	// seek original file back to 0
	mock.EXPECT().Seek(gomock.Eq(int64(0)), gomock.Eq(0)).Return(int64(0), nil)
}

func TestFileBufferInit(t *testing.T) {
	t.Run("is able to use a symlink file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fileName := "elMeuNom"
		data := []byte("bon dia senyor")
		fileInfo := testFileInfo{mode: os.ModeSymlink}

		f, mock := newTestFileBuffer(ctrl)

		expectInitSwap(mock, fileName, fileInfo, data)
		expectInitBuffer(mock, data)

		assert.NoError(t, f.Init(fileName, cell.NewBuffer(), ""))
	})

	t.Run("is able to use a regular file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		fileName := "myName"
		data := []byte("good morning sir")
		fileInfo := testFileInfo{}

		f, mock := newTestFileBuffer(ctrl)
		expectInitSwap(mock, fileName, fileInfo, data)
		expectInitBuffer(mock, data)

		assert.NoError(t, f.Init(fileName, cell.NewBuffer(), ""))
	})

	t.Run("returns error if original file is not regular or symlink file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock := newTestFileBuffer(ctrl)
		mock.EXPECT().Stat().Return(testFileInfo{mode: os.ModeSocket}, nil)

		assert.Equal(t, ErrFileIsNotRegular, f.Init("fjkelw", cell.NewBuffer(), ""))
	})

	t.Run("returns error if original file is directory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock := newTestFileBuffer(ctrl)
		mock.EXPECT().Stat().Return(testFileInfo{isDir: true}, nil)

		assert.Equal(t, ErrFileIsNotRegular, f.Init("fjkelw", cell.NewBuffer(), ""))
	})

	t.Run("bubble up original file open error", func(t *testing.T) {
		accessDeniedErr := errors.New("access denied")
		f := new(FileBuffer)
		f.openFunc = func(name string, flag int, perm os.FileMode) (osFile, error) {
			return nil, accessDeniedErr
		}
		assert.Equal(t, accessDeniedErr, f.Init("fjkelw", cell.NewBuffer(), ""))
	})

	t.Run("bubble up swap file open error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		accessDeniedErr := errors.New("access denied")
		origFileMock := NewMockOsFile(ctrl)
		f := new(FileBuffer)
		i := 0
		f.openFunc = func(name string, flag int, perm os.FileMode) (osFile, error) {
			i++
			if i == 1 {
				return origFileMock, nil
			}
			return nil, accessDeniedErr
		}

		fileName := "fjklewjflk"

		origFileMock.EXPECT().Stat().Return(testFileInfo{}, nil)
		origFileMock.EXPECT().Name().Return(fileName).AnyTimes()

		assert.Equal(t, accessDeniedErr, f.Init(fileName, cell.NewBuffer(), ""))
	})
}

const defaultFileName = "myOhDear.go"

var defaultFileData = []byte("oh, dear")

func newInitializedTestFileBuffer(t *testing.T, ctrl *gomock.Controller) (
	*FileBuffer, *MockOsFile, *cell.Buffer,
) {
	f, mock := newTestFileBuffer(ctrl)
	expectInitSwap(mock, defaultFileName, testFileInfo{}, defaultFileData)
	expectInitBuffer(mock, defaultFileData)
	buf := cell.NewBuffer()
	require.NoError(t, f.Init(defaultFileName, buf, ""))
	return f, mock, buf
}

func newUninitializedTestFileBuffer(t *testing.T, ctrl *gomock.Controller) (
	*FileBuffer, *MockOsFile, *cell.Buffer,
) {
	f, mock := newTestFileBuffer(ctrl)
	f.openFunc = func(name string, flag int, perm os.FileMode) (osFile, error) {
		if flag&os.O_CREATE != 0 {
			return mock, nil
		}
		return nil, &os.PathError{Err: os.ErrNotExist}
	}

	mock.EXPECT().Name().Return(defaultFileName).AnyTimes()

	buf := cell.NewBuffer()
	require.NoError(t, f.Init(defaultFileName, buf, ""))
	return f, mock, buf
}

func newRecoveredTestFileBuffer(t *testing.T, ctrl *gomock.Controller) (
	*FileBuffer, *MockOsFile, *cell.Buffer,
) {
	f, mock := newTestFileBuffer(ctrl)
	mock.EXPECT().Stat().Return(testFileInfo{}, nil).Times(3)
	mock.EXPECT().Close().Return(nil).Times(2)
	expectInitSwap(mock, defaultFileName, testFileInfo{}, defaultFileData)
	expectInitBuffer(mock, defaultFileData)
	buf := cell.NewBuffer()
	require.NoError(t, f.recoverFile(defaultFileName, defaultFileName+".swp", buf))
	return f, mock, buf
}

func testFileBufferClose(t *testing.T, newBuffer newBufferFunc) {
	t.Run("Close should remove and close all resources such that Init can be called again", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newBuffer(t, ctrl)

		mock.EXPECT().Close().Return(nil).Times(2)
		assert.NoError(t, f.Close())

		expectInitSwap(mock, defaultFileName, testFileInfo{}, defaultFileData)
		expectInitBuffer(mock, defaultFileData)
		assert.NoError(t, f.Init(defaultFileName, cell.NewBuffer(), ""))
	})

	t.Run("two consecutive calls to Close should return an error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newBuffer(t, ctrl)

		mock.EXPECT().Close().Return(nil).Times(2)
		assert.NoError(t, f.Close())
		assert.Error(t, f.Close())
	})

	t.Run("Close should remove the swap file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newBuffer(t, ctrl)
		called := false
		f.removeFunc = func(name string) error {
			called = true
			return nil
		}

		mock.EXPECT().Close().Return(nil).Times(2)
		assert.NoError(t, f.Close())
		assert.True(t, called)
	})
}

func TestNewFileBufferClose(t *testing.T) {
	testFileBufferClose(t, newInitializedTestFileBuffer)
}

func TestRecoverFileBufferClose(t *testing.T) {
	testFileBufferClose(t, newRecoveredTestFileBuffer)
}

func TestFileNotCreatedBufferClose(t *testing.T) {
	t.Run("Close should remove and close all resources such that Init can be called again", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newUninitializedTestFileBuffer(t, ctrl)

		mock.EXPECT().Close().Return(nil).Times(1)
		assert.NoError(t, f.Close())

		assert.NoError(t, f.Init(defaultFileName, cell.NewBuffer(), ""))
	})

	t.Run("two consecutive calls to Close should return an error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newUninitializedTestFileBuffer(t, ctrl)

		mock.EXPECT().Close().Return(nil).Times(1)
		assert.NoError(t, f.Close())
		assert.Error(t, f.Close())
	})
}

func testFileBufferFlush(t *testing.T, newBuffer newBufferFunc) {
	t.Run("Flush bubbles up Stat errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newBuffer(t, ctrl)

		myErr := errors.New("what?")
		mock.EXPECT().Stat().Return(nil, myErr)
		assert.Equal(t, myErr, f.Flush())
	})

	t.Run("flush syncs the contents of the buffer to disk", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newBuffer(t, ctrl)
		called := false
		f.renameFunc = func(oldName, newName string) error {
			called = true
			return nil
		}

		mock.EXPECT().Stat().Return(testFileInfo{}, nil)
		mock.EXPECT().Close().Return(nil).Times(2)
		expectInitSwap(mock, defaultFileName, testFileInfo{}, defaultFileData)

		assert.NoError(t, f.Flush())
		assert.True(t, called)
	})

	t.Run("flush bubbles up rename errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newBuffer(t, ctrl)
		myErr := errors.New("wtf")
		f.renameFunc = func(oldName, newName string) error {
			return myErr
		}

		mock.EXPECT().Stat().Return(testFileInfo{}, nil)

		assert.Equal(t, myErr, f.Flush())
	})

	t.Run("flush returns error if file was modified", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, _ := newBuffer(t, ctrl)
		mock.EXPECT().Stat().Return(testFileInfo{modTime: time.Now()}, nil)
		assert.Error(t, ErrStaleData, f.Flush())
	})
}

func TestNewFileBufferFlush(t *testing.T) {
	testFileBufferFlush(t, newInitializedTestFileBuffer)
}

func TestRecoverFileBufferFlush(t *testing.T) {
	testFileBufferFlush(t, newRecoveredTestFileBuffer)
}

func TestFileNotCreatedBufferFlush(t *testing.T) {
	t.Run("if file is created after NewFileBuffer is called returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, _, _ := newUninitializedTestFileBuffer(t, ctrl)
		require.Nil(t, f.orig)

		f.openFunc = func(name string, flag int, perm os.FileMode) (osFile, error) {
			assert.NotZero(t, flag&os.O_CREATE)
			return nil, &os.PathError{Err: os.ErrExist}
		}
		assert.Equal(t, ErrStaleData, f.Flush())
	})
}

func expectCopyToSwapPrepare(mock *MockOsFile) {
	mock.EXPECT().Truncate(gomock.Eq(int64(0))).Return(nil)
	mock.EXPECT().Seek(gomock.Eq(int64(0)), gomock.Eq(0)).Return(int64(0), nil)
	mock.EXPECT().
		Write(gomock.Eq([]byte("\n"))).
		Return(1, nil)
	mock.EXPECT().
		Sync().
		Return(nil)
}

func expectCopyToSwap(mock *MockOsFile, newData string) {
	expectedContent := newData + string(defaultFileData)
	expectCopyToSwapPrepare(mock)
	mock.EXPECT().
		WriteString(gomock.Eq(expectedContent)).
		Return(len(expectedContent), nil)
}

type newBufferFunc func(*testing.T, *gomock.Controller) (*FileBuffer, *MockOsFile, *cell.Buffer)

func testFileBufferOp(
	t *testing.T,
	newBuffer newBufferFunc,
	op func(*cell.Buffer, string),
) {

	t.Run("if copy to swap fails, retries on next insert", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		_, mock, buf := newBuffer(t, ctrl)
		myString := "my string\n"
		myError := errors.New("I feel clammy")

		mock.EXPECT().Truncate(gomock.Eq(int64(0))).Return(myError)
		buf.InsertString(term.Coordinates{}, myString)

		expectCopyToSwap(mock, myString+myString)
		buf.InsertString(term.Coordinates{}, myString)
	})

	t.Run("if copy to swap fails, retries on next Flush", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, buf := newBuffer(t, ctrl)
		myString := "Sant Hipòlit de Voltregà"
		myError := errors.New("No té capità")

		mock.EXPECT().Truncate(gomock.Eq(int64(0))).Return(nil)
		mock.EXPECT().Seek(gomock.Eq(int64(0)), gomock.Eq(0)).Return(int64(0), myError)
		buf.InsertString(term.Coordinates{}, myString)

		expectCopyToSwap(mock, myString)
		mock.EXPECT().Stat().Return(testFileInfo{}, nil)
		mock.EXPECT().Close().Return(nil).Times(2)
		expectInitSwap(mock, defaultFileName, testFileInfo{}, defaultFileData)
		assert.NoError(t, f.Flush())
	})

	t.Run("if copy to swap fails, retries on next Flush and fails, bubbles up error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, buf := newBuffer(t, ctrl)
		myString := "Ballz"
		myError := errors.New("rounder")

		mock.EXPECT().Truncate(gomock.Eq(int64(0))).Return(myError)
		buf.InsertString(term.Coordinates{}, myString)

		mock.EXPECT().Truncate(gomock.Eq(int64(0))).Return(myError)
		assert.Error(t, f.Flush())
	})

	t.Run("if copy to swap fails, errors contains details of swap file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, buf := newBuffer(t, ctrl)
		myString := "a"
		myError := errors.New("access super-denied")

		mock.EXPECT().Truncate(gomock.Eq(int64(0))).Return(myError)
		buf.InsertString(term.Coordinates{}, myString)

		mock.EXPECT().Truncate(gomock.Eq(int64(0))).Return(myError)
		err := f.Flush()
		require.Error(t, err)
		assert.Contains(t, err.Error(), defaultFileName)
	})
}

func testFileBufferDelete(t *testing.T, newBuffer newBufferFunc) {

	testFileBufferOp(t, newBuffer, func(buf *cell.Buffer, data string) {
		buf.InsertString(term.Coordinates{}, data)
	})

	t.Run("upon Delete, it copies content to swap file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		_, mock, buf := newBuffer(t, ctrl)
		expectCopyToSwapPrepare(mock)
		mock.EXPECT().
			WriteString(gomock.Eq("")).
			Return(len(""), nil)
		buf.DeleteRow(0)
	})
}

func testFileBufferInsert(t *testing.T, newBuffer newBufferFunc) {
	testFileBufferOp(t, newBuffer, func(buf *cell.Buffer, data string) {
		buf.DeleteRow(0)
	})

	t.Run("upon Insert, it copies content to swap file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, buf := newBuffer(t, ctrl)
		require.Equal(t, string(defaultFileData), buf.String())
		require.Equal(t, string(defaultFileData), f.reader.String())

		myString := "my string\n"
		expectCopyToSwap(mock, myString)

		buf.InsertString(term.Coordinates{}, myString)
	})
}

func TestNewFileBufferDelete(t *testing.T) {
	testFileBufferDelete(t, newInitializedTestFileBuffer)
}

func TestNewFileBufferInsert(t *testing.T) {
	testFileBufferInsert(t, newInitializedTestFileBuffer)
}

func TestRecoverFileBufferDelete(t *testing.T) {
	testFileBufferDelete(t, newRecoveredTestFileBuffer)
}

func TestRecoverFileBufferInsert(t *testing.T) {
	testFileBufferInsert(t, newRecoveredTestFileBuffer)
}

func testFileBufferFlushed(t *testing.T, newBuffer newBufferFunc) {
	t.Run("Flushed should return true upon initialization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, _, _ := newBuffer(t, ctrl)
		assert.True(t, f.Flushed())
	})

	t.Run("Flushed should return false after insert is called", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, buf := newBuffer(t, ctrl)

		myString := "fjklew"
		expectCopyToSwap(mock, myString)
		buf.InsertString(term.Coordinates{}, myString)

		assert.False(t, f.Flushed())
	})

	t.Run("Flushed should return false after delete is called", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, buf := newBuffer(t, ctrl)

		myString := "fjklew"
		expectCopyToSwap(mock, myString)
		buf.DeleteRow(0)

		assert.False(t, f.Flushed())
	})

	t.Run("Flushed should return false after delete is called", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		f, mock, buf := newBuffer(t, ctrl)

		myString := ""
		expectCopyToSwap(mock, myString)
		buf.DeleteRow(0)

		require.NoError(t, f.Flush())
		assert.True(t, f.Flushed())
	})
}
