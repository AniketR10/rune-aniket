// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package textrpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
	"unstable.build/rune/internal/cell"
	"unstable.build/rune/internal/component"
	"unstable.build/rune/internal/text"
	"unstable.build/rune/internal/text/texttest"
)

var (
	uri      workspaceapi.URI
	protoURI textrpc.URI
)

func init() {
	var err error
	uri, err = workspaceapi.ParseURI("file:///test")
	if err != nil {
		panic(err)
	}

	protoURI = *NewURI(uri)
}

func makeEventIntegrationCase(content string) (in, out *cell.Buffer, cursor *text.Cursor) {
	in, out = cell.NewBuffer(), cell.NewBuffer()
	scroll := component.NewScroll(in)
	scroll.Resize(100, 100)
	cursor = text.NewCursor(scroll, nil)
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
	cursor.InsertLineBelow(text.IndentRuneTab, 0)
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
	require.True(t, cursor.MoveEndLine())

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
				out.Edit(ctx, ev.Start, ev.End, ev.Content)
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
		out textrpc.EditorEvent
	}{
		{
			in: textapi.Event{
				Type:    textapi.EventTypeOpen,
				Content: "Ageispolis",
			},
			out: textrpc.EditorEvent{
				Type:    textrpc.EditorEvent_TypeOpen,
				Content: "Ageispolis",
			},
		},
		{
			in: textapi.Event{
				Type:  textapi.EventTypeHidden,
				Start: term.Coordinates{Y: 8},
				End:   term.Coordinates{Y: 9},
			},
			out: textrpc.EditorEvent{
				Type:  textrpc.EditorEvent_TypeHidden,
				Start: &termrpc.Coordinates{Y: 8},
				End:   &termrpc.Coordinates{Y: 9},
			},
		},
		{
			in: textapi.Event{
				Type:  textapi.EventTypeVisible,
				Start: term.Coordinates{Y: 8},
			},
			out: textrpc.EditorEvent{
				Type:  textrpc.EditorEvent_TypeVisible,
				Start: &termrpc.Coordinates{Y: 8},
			},
		},
		{
			in: textapi.Event{
				Type: textapi.EventTypeFocus,
				URI:  uri,
				Resource: textrpc.Token{
					URI: uri,
				},
			},
			out: textrpc.EditorEvent{
				Type:         textrpc.EditorEvent_TypeFocus,
				ResourceName: &protoURI,
			},
		},
		{
			in: textapi.Event{
				Type:     textapi.EventTypeUnfocus,
				URI:      uri,
				Resource: textrpc.Token{URI: uri},
			},
			out: textrpc.EditorEvent{
				Type:         textrpc.EditorEvent_TypeUnfocus,
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
			out: textrpc.EditorEvent{
				Type:  textrpc.EditorEvent_TypeScroll,
				Start: &termrpc.Coordinates{X: 1, Y: 2},
				End:   &termrpc.Coordinates{X: 3, Y: 4},
				From:  &termrpc.Coordinates{X: 5, Y: 6},
				To:    &termrpc.Coordinates{X: 7, Y: 8},
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

func assertEqualProto(t *testing.T, expected, actual textrpc.EditorEvent) {
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
