package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
	texttest "unstable.build/go-tui/text/test"
	"unstable.build/go-tui/workspace"
)

var (
	uri      workspace.URI
	protoURI URI
)

func init() {
	var err error
	uri, err = workspace.ParseURI("file:///test")
	if err != nil {
		panic(err)
	}

	protoURI = *NewURI(uri)
}

func makeEventIntegrationCase(content string) (in, out *cell.Buffer, cursor *text.Cursor) {
	in, out = cell.NewBuffer(), cell.NewBuffer()
	scroll := component.NewScroll(in)
	scroll.Resize(100, 100)
	cursor = text.NewCursor(scroll)
	in.WriteString(content)
	out.WriteString(content)
	return
}

func TestIntegrationInsert(t *testing.T) {
	in, out, cursor := makeEventIntegrationCase("")
	in.Subscribe(text.CellSubscriber(uri, texttest.NewTestHandler(),
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
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

	require.True(t, cursor.MoveRight())
	cursor.Insert('\n')
	cursor.Insert('\n')
	cursor.Insert('i')
	cursor.Insert('f')
	cursor.Insert('{')
	cursor.Insert('\n')
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

	in.Subscribe(text.CellSubscriber(uri, texttest.NewTestHandler(),
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			out.Delete(ev.Start, ev.End)
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

	in.Subscribe(text.CellSubscriber(uri, texttest.NewTestHandler(),
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			switch ev.Type {
			case textapi.EventTypeEdit:
				out.Edit(ev.Start, ev.End, ev.Content)
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
		in  textapi.Event
		out EditorEvent
	}{
		{
			in: textapi.Event{
				Type:    textapi.EventTypeOpen,
				Content: "Ageispolis",
			},
			out: EditorEvent{
				Type:    EditorEvent_TypeOpen,
				Content: "Ageispolis",
			},
		},
		{
			in: textapi.Event{
				Type: textapi.EventTypeFocus,
				URI:  uri,
				Resource: Token{
					URI: uri,
				},
			},
			out: EditorEvent{
				Type:         EditorEvent_TypeFocus,
				ResourceName: &protoURI,
			},
		},
		{
			in: textapi.Event{
				Type:     textapi.EventTypeUnfocus,
				URI:      uri,
				Resource: Token{URI: uri},
			},
			out: EditorEvent{
				Type:         EditorEvent_TypeUnfocus,
				ResourceName: &protoURI,
			},
		},
		{
			in: textapi.Event{
				Type:  textapi.EventTypeScroll,
				Start: term.Coordinates{X: 1, Y: 2},
				End:   term.Coordinates{X: 3, Y: 4},
				From:  term.Coordinates{X: 5, Y: 6},
				To:    term.Coordinates{X: 7, Y: 8},
			},
			out: EditorEvent{
				Type:  EditorEvent_TypeScroll,
				Start: &termpb.Coordinates{X: 1, Y: 2},
				End:   &termpb.Coordinates{X: 3, Y: 4},
				From:  &termpb.Coordinates{X: 5, Y: 6},
				To:    &termpb.Coordinates{X: 7, Y: 8},
			},
		},
	}

	t.Run("editor.Event -> EditorEvent", func(t *testing.T) {
		for _, tcase := range tsuite {
			actual := toProto(tcase.in)
			assertEqualProto(t, tcase.out, actual)
		}
	})

	t.Run("EditorEvent -> editor.Event ", func(t *testing.T) {
		for _, tcase := range tsuite {
			actual := textapi.Event{}
			fromProto(&actual, &tcase.out)
			assertEqualEvent(t, tcase.in, actual)
		}
	})
}

func assertEqualProto(t *testing.T, expected, actual EditorEvent) {
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.ResourceName.GetUri(), actual.ResourceName.GetUri())
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

func assertEqualEvent(t *testing.T, expected, actual textapi.Event) {
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.URI.String(), actual.URI.String())
	assert.Equal(t, expected.Resource, actual.Resource)
	assert.Equal(t, expected.Start, actual.Start)
	assert.Equal(t, expected.From, actual.From)
	assert.Equal(t, expected.Content, actual.Content)
}
