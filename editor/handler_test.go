package editor

import (
	"os"
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEditor struct {
	buf *cell.Buffer
}

func (e *testEditor) Edit(buf *cell.Buffer) fractal.Handler {
	e.buf = buf
	return handler.NewTestHandler()
}

func openTestFile(filePath string, buf *cell.Buffer, swapDir string) (
	*FileBuffer, error,
) {
	f := new(FileBuffer)
	f.openFunc = func(name string, flag int, perm os.FileMode) (osFile, error) {
		return &noopFile{name: filePath}, nil
	}
	f.removeFunc = func(name string) error {
		return nil
	}
	f.renameFunc = func(oldName, newName string) error {
		return nil
	}
	f.statFunc = func(name string) (os.FileInfo, error) {
		return testFileInfo{}, nil
	}
	return f, nil
}

func recoverTestFile(filePath, swapFilePath string, buf *cell.Buffer) (
	*FileBuffer, error,
) {
	return openTestFile(filePath, buf, "")
}

func newTestEditorHandler() *editorHandler {
	ret := new(editorHandler)
	ret.openFileFn = openTestFile
	ret.recoverFileFn = recoverTestFile
	return ret
}

func TestEditorHandlerDraw(t *testing.T) {
	cases := []handler.TestInputSequence{
		{"asdf",
			`┌──────────────────┐
│                  │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
└──────────────────┘`},
		{":",
			`┌──────────────────┐
│                  │
├──────────────────┤
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│EEEEEEEEEEEEEEEEEE│
│▐                 │
└──────────────────┘`},
		{"e cabin.go>",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"a",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":e other.go>",
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"#", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"#", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{"$", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{"$", // simulates ctrl-h
			`┌──────────────────┐
│cabin.go  other.go│
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`},
		{":close>",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":close>",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│Error: No free buf│
└──────────────────┘`},
		{":wq!^",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│wq▐               │
└──────────────────┘`},
		{"^^^^",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
		{":<",
			`┌──────────────────┐
│cabin.go          │
├──────────────────┤
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`},
	}

	ed := newTestEditorHandler()
	err := ed.Init(&testEditor{}, WithFilepath("")) //first handler is empty handler
	require.NoError(t, err)
	handler.BatchTestInputSequence(t, ed, 20, 10, cases)

	assert.NoError(t, ed.Close())
}
