package editor

import (
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeEventIntegrationCase(content string) (in, out *cell.Buffer, cursor *Cursor) {
	in, out = cell.NewBuffer(), cell.NewBuffer()
	scroll := component.NewScroll()
	scroll.InitWithBuffer(in)
	scroll.Resize(100, 100)
	cursor = NewCursor(scroll)
	in.WriteString(content)
	out.WriteString(content)
	return
}

func TestIntegrationInsert(t *testing.T) {
	in, out, cursor := makeEventIntegrationCase("")
	in.Subscribe(CellSubscriber("", handler.NewTestHandler(), FuncEventHandler(func(ev Event) bool {
		out.InsertString(ev.Start, ev.Content)
		return false
	})))

	content := `package main
func main() {
}`
	in.WriteString(content)
	assert.Equal(t, out.String(), content)

	require.True(t, cursor.MoveDown())
	cursor.InsertRowBelow()
	cursor.Insert('\t')
	cursor.Insert('f')
	cursor.Insert('m')
	cursor.Insert('t')
	cursor.Insert('.')

	assert.Equal(t, `package main
func main() {
	fmt.
}`, out.String())

	require.True(t, cursor.MoveLastLine())

	cursor.InsertRowBelow()
	cursor.InsertRowBelow()
	cursor.MoveStartLine()
	cursor.Insert('i')
	cursor.Insert('f')
	cursor.Insert('{')
	cursor.InsertRowBelow()
	cursor.MoveStartLine()
	cursor.Insert('\t')
	cursor.Insert('X')
	cursor.Insert('\n')
	cursor.Insert('}')

	assert.Equal(t, `package main
func main() {
	fmt.
}

if{
	X
}`, out.String())

	// one of the newlines is "absorbed" by the unix file reader
	in.InsertString(term.Coordinates{Y: 10}, "boom\n\n")

	assert.Equal(t, `package main
func main() {
	fmt.
}

if{
	X
}


boom
`, out.String())
}

func TestIntegrationDelete(t *testing.T) {
	content := `package main
func main() {
	for {
		fmt.Println("Six")
	}
}`
	in, out, cursor := makeEventIntegrationCase(content)

	in.Subscribe(CellSubscriber("", handler.NewTestHandler(), FuncEventHandler(func(ev Event) bool {
		out.Delete(ev.From, ev.To)
		return false
	})))

	require.True(t, cursor.MoveDown())
	require.True(t, cursor.MoveDown())
	assert.True(t, cursor.SelectLine())
	require.True(t, cursor.MoveDown())
	require.True(t, cursor.MoveDown())
	assert.True(t, cursor.DeleteSelection())

	assert.Equal(t, `package main
func main() {
}`, out.String())
}

func TestIntegrationUndoer(t *testing.T) {
	content := `package main
func main() {
	for {
		fmt.Println("Six")
	}
}`
	in, out, cursor := makeEventIntegrationCase(content)

	in.Subscribe(CellSubscriber("", handler.NewTestHandler(), FuncEventHandler(func(ev Event) bool {
		switch ev.Type {
		case EventTypeInsert:
			out.InsertString(ev.Start, ev.Content)
		case EventTypeDelete:
			out.Delete(ev.From, ev.To)
		}
		return false
	})))

	require.True(t, cursor.MoveDown())
	require.True(t, cursor.MoveDown())
	assert.True(t, cursor.Select())
	require.True(t, cursor.MoveDown())
	require.True(t, cursor.MoveDown())
	require.True(t, cursor.MoveEndLine())
	assert.True(t, cursor.DeleteSelection())

	modified := `package main
func main() {

}`
	assert.Equal(t, modified, out.String())

	cursor.Undo()
	assert.Equal(t, content, out.String())

	cursor.Redo()
	assert.Equal(t, modified, out.String())
}
