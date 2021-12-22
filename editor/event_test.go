package editor

import (
	"testing"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/proto"
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
	in.Subscribe(CellSubscriber("", NewTestHandler(),
		FuncEventHandler(func(ev Event) bool {
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

	in.Subscribe(CellSubscriber("", NewTestHandler(),
		FuncEventHandler(func(ev Event) bool {
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

	in.Subscribe(CellSubscriber("", NewTestHandler(),
		FuncEventHandler(func(ev Event) bool {
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

func TestEventProto(t *testing.T) {
	tsuite := []struct {
		in  Event
		out proto.EditorEvent
	}{
		{
			in: Event{
				Type:         EventTypeOpen,
				ResourceName: "Aphex Twin",
				Resource:     nil,
				Content:      "Ageispolis",
			},
			out: proto.EditorEvent{
				Type:         proto.EditorEvent_TypeOpen,
				ResourceName: "Aphex Twin",
				ResourceId:   0,
				Content:      "Ageispolis",
			},
		},
		{
			in: Event{
				Type:         EventTypeFocus,
				ResourceName: "Ambient works",
				Resource: Token{Token: browser.Token{ID: 2},
					resource: "Ambient works"},
			},
			out: proto.EditorEvent{
				Type:         proto.EditorEvent_TypeFocus,
				ResourceName: "Ambient works",
				ResourceId:   2,
			},
		},
		{
			in: Event{
				Type:  EventTypeScroll,
				Start: term.Coordinates{X: 1, Y: 2},
				End:   term.Coordinates{X: 3, Y: 4},
				From:  term.Coordinates{X: 5, Y: 6},
				To:    term.Coordinates{X: 7, Y: 8},
			},
			out: proto.EditorEvent{
				Type:  proto.EditorEvent_TypeScroll,
				Start: &proto.Coordinates{X: 1, Y: 2},
				End:   &proto.Coordinates{X: 3, Y: 4},
				From:  &proto.Coordinates{X: 5, Y: 6},
				To:    &proto.Coordinates{X: 7, Y: 8},
			},
		},
	}

	t.Run("editor.Event -> proto.EditorEvent", func(t *testing.T) {
		for _, tcase := range tsuite {
			actual := tcase.in.toProto()
			assertEqualProto(t, tcase.out, actual)
		}
	})

	t.Run("proto.EditorEvent -> editor.Event ", func(t *testing.T) {
		for _, tcase := range tsuite {
			actual := Event{}
			actual.fromProto(&tcase.out)
			assertEqualEvent(t, tcase.in, actual)
		}
	})
}

func assertEqualProto(t *testing.T, expected, actual proto.EditorEvent) {
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.ResourceName, actual.ResourceName)
	assert.Equal(t, expected.ResourceId, actual.ResourceId)
	assert.Equal(t, expected.Start.GetX(), actual.Start.GetX())
	assert.Equal(t, expected.Start.GetY(), actual.Start.GetY())
	assert.Equal(t, expected.End.GetX(), actual.End.GetX())
	assert.Equal(t, expected.End.GetY(), actual.End.GetY())
	assert.Equal(t, expected.From.GetX(), actual.From.GetX())
	assert.Equal(t, expected.From.GetY(), actual.From.GetY())
	assert.Equal(t, expected.To.GetX(), actual.To.GetX())
	assert.Equal(t, expected.To.GetY(), actual.To.GetY())
	assert.Equal(t, expected.Content, actual.Content)
}

func assertEqualEvent(t *testing.T, expected, actual Event) {
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.ResourceName, actual.ResourceName)
	assert.Equal(t, expected.Resource, actual.Resource)
	assert.Equal(t, expected.Start, actual.Start)
	assert.Equal(t, expected.From, actual.From)
	assert.Equal(t, expected.Content, actual.Content)
}
