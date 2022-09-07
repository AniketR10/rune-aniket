package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/debug"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text/vi"
	"github.com/ernestrc/go-tui/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newIntegrationTestCase(t *testing.T, content string) (
	*cell.Buffer, workspace.FlusherCloser, workspace.URI, func(),
) {
	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	workspaceURI, err := workspace.CurrentUserHostURI(tempDir)
	require.NoError(t, err)

	manager, err := workspace.NewManager(debug.StandardLogger(), workspaceURI)
	require.NoError(t, err)

	file, err := ioutil.TempFile(tempDir, "workspace_int_test")
	require.NoError(t, err)

	_, err = file.Write([]byte(content))
	require.NoError(t, err)

	uri, err := workspace.CurrentUserHostURI(file.Name())
	require.NoError(t, err)

	swapURI, err := workspace.CurrentUserHostURI(tempDir)
	require.NoError(t, err)

	buffer := cell.NewBuffer()
	fc, err := manager.Open(uri, buffer, swapURI, false)
	require.NoError(t, err)

	return buffer, fc, uri, func() {
		file.Close()
		os.Remove(file.Name())
	}
}

func TestLastEOLUndoFileIntegration(t *testing.T) {
	buf, _, _, cleanup := newIntegrationTestCase(t, "a\n")
	defer cleanup()

	initialString := buf.String()
	initialCells := buf.RawCells()
	assert.Equal(t, "a", initialString)
	assert.Equal(t,
		[][]term.Cell{{{Ch: 'a'}}}, initialCells)

	buf.InsertRowAt(1)
	newString := buf.String()
	newCells := buf.RawCells()
	assert.Equal(t, "a\n", newString)
	assert.Equal(t,
		[][]term.Cell{{{Ch: 'a'}}, []term.Cell{}}, newCells)

	ok, _ := buf.Undo()
	assert.True(t, ok)
	newString2 := buf.String()
	newCells2 := buf.RawCells()
	assert.Equal(t, initialString, newString2)
	assert.Equal(t, initialCells, newCells2)

	buf.InsertRowAt(1)
	newString = buf.String()
	newCells = buf.RawCells()
	assert.Equal(t, "a\n", newString)
	assert.Equal(t,
		[][]term.Cell{{{Ch: 'a'}}, []term.Cell{}}, newCells)
}

func TestIntegrationVi(t *testing.T) {
	t.Run("last EOL", func(t *testing.T) {
		for _, content := range []string{"hello", "hello\n"} {
			t.Run(fmt.Sprintf("insert word below last line: %q", content), func(t *testing.T) {
				buf, _, uri, cleanup := newIntegrationTestCase(t, content)
				defer cleanup()

				vi := vi.New(buf, uri)
				vi.Resize(4, 4)

				for _, ch := range "Goworld" {
					vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
				vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
				assert.Equal(t, "hello\nworld", buf.String())

				// undo
				_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'u'})
				require.True(t, handled)
				require.Equal(t, "hello", buf.String())

				for _, ch := range "Goworld" {
					vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
				vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
				assert.Equal(t, "hello\nworld", buf.String())
			})

			t.Run(fmt.Sprintf("insert a newline last line: %q", content), func(t *testing.T) {
				buf, _, uri, clean := newIntegrationTestCase(t, content)
				defer clean()

				vi := vi.New(buf, uri)
				vi.Resize(4, 4)

				for _, ch := range "Go\n" {
					vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
				vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
				assert.Equal(t, "hello\n\n", buf.String())

				// undo
				_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: 'u'})
				require.True(t, handled)
				require.Equal(t, "hello", buf.String())

				for _, ch := range "Go\n" {
					vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
				vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
				assert.Equal(t, "hello\n\n", buf.String())
			})
		}
	})
}
