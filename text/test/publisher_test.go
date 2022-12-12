package test

import (
	"context"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

func newEdit(content string) (*cell.Buffer, *component.Scroll, *text.Cursor) {
	buf := cell.NewBuffer()
	buf.WriteString(content)
	scroll := component.NewScroll(buf)
	cursor := text.NewCursor(scroll)
	return buf, scroll, cursor
}

func newMock(ctrl *gomock.Controller) *MockHandler {
	mock := NewMockHandler(ctrl)
	mock.EXPECT().Resize(gomock.Any(), gomock.Any()).AnyTimes()
	return mock
}

func TestPublisher(t *testing.T) {
	uri, err := workspace.ParseURI("file:///Teamshares")
	require.NoError(t, err)
	t.Run("publishes EventTypeOpen when PublishEdit is called", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		pub := text.NewPublisher()
		content := "1. She interviews"

		var eventHandler text.Handler
		pub.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeOpen},
			text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				assert.Equal(t, textapi.EventTypeOpen, ev.Type)
				assert.Equal(t, content, ev.Content)
				assert.Equal(t, uri, ev.URI)
				eventHandler = ev.Resource
				return false
			}))

		buf, _, cursor := newEdit(content)
		returnedHandler := pub.PublishEdit(uri, buf, newMock(ctrl), cursor)
		assert.Equal(t, eventHandler, returnedHandler)
	})

	t.Run("publishes EventTypeFocus when PublishEdit is called", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		pub := text.NewPublisher()
		content := "2. She gets hired"

		var eventHandler text.Handler
		pub.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeFocus},
			text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				assert.Equal(t, textapi.EventTypeFocus, ev.Type)
				assert.Equal(t, uri, ev.URI)
				eventHandler = ev.Resource
				return false
			}))

		buf, _, cursor := newEdit(content)
		returnedHandler := pub.PublishEdit(uri, buf, newMock(ctrl), cursor)
		assert.Equal(t, eventHandler, returnedHandler)
	})

	t.Run("subscribes to multiple events", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		pub := text.NewPublisher()
		content := "3. SHE GOT HIRED, I KNEW IT!!!"

		var eventHandler text.Handler
		var fired int
		pub.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeOpen, textapi.EventTypeFocus},
			text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				fired++
				assert.Equal(t, uri, ev.URI)
				eventHandler = ev.Resource
				return false
			}))

		buf, _, cursor := newEdit(content)
		returnedHandler := pub.PublishEdit(uri, buf, newMock(ctrl), cursor)
		assert.Equal(t, eventHandler, returnedHandler)
		assert.Equal(t, 2, fired)
	})

	t.Run("dispatches EventTypeCursor when given cursor changes on Handle", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		pub := text.NewPublisher()
		mock := newMock(ctrl)
		buf, _, cursor := newEdit("\n\n")

		h := pub.PublishEdit(uri, buf, mock, cursor)
		h.Resize(2, 2)

		var called bool
		pub.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeCursor},
			text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				require.Equal(t, textapi.EventTypeCursor, ev.Type)
				assert.Equal(t, uri, ev.URI)
				assert.Equal(t, h, ev.Resource)
				assert.Equal(t, term.Coordinates{Y: 2}, ev.Start)
				assert.Equal(t, term.Coordinates{}, ev.From)
				called = true
				return false
			}))

		mock.EXPECT().Handle(gomock.Any()).
			DoAndReturn(func(ev term.Event) (bool, bool) {
				require.Equal(t, term.Event{Type: term.EventInterrupt}, ev)
				return false, true
			}).Times(1)

		var times int
		mock.EXPECT().Cursor().
			DoAndReturn(func() (term.Coordinates, bool) {
				if times == 0 {
					times++
					return term.Coordinates{Y: 1}, true
				}
				return term.Coordinates{Y: 2}, true
			}).Times(2)

		exit, handled := h.Handle(term.Event{Type: term.EventInterrupt})
		assert.False(t, exit)
		assert.True(t, handled)
		assert.True(t, called)
	})

	t.Run("dispatches EventTypeScroll when scroll seeks", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pub := text.NewPublisher()
		buf, scroll, cursor := newEdit("\n\n")

		h := pub.PublishEdit(uri, buf, newMock(ctrl), cursor)
		h.Resize(2, 2)
		require.True(t, scroll.SeekDown())

		at := term.Coordinates{X: -1}
		pub.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeScroll},
			text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				require.Equal(t, textapi.EventTypeScroll, ev.Type)
				assert.Equal(t, uri, ev.URI)
				assert.Equal(t, h, ev.Resource)
				at = ev.Start
				return false
			}))

		require.True(t, scroll.SeekUp())
		assert.Equal(t, term.Coordinates{}, at)

		at = term.Coordinates{X: -1}
		require.True(t, scroll.SeekDown())
		assert.Equal(t, term.Coordinates{Y: 1}, at)
	})

	t.Run("dispatches EventTypeEdit when buffer content is inserted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pub := text.NewPublisher()
		buf, _, cursor := newEdit("\n\n")

		h := pub.PublishEdit(uri, buf, newMock(ctrl), cursor)
		h.Resize(2, 2)

		var called bool
		pub.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeEdit},
			text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				called = true
				require.Equal(t, textapi.EventTypeEdit, ev.Type)
				assert.Equal(t, term.Coordinates{Y: 2}, ev.Start, "Start")
				assert.Equal(t, term.Coordinates{Y: 2}, ev.End, "End")
				assert.Equal(t, term.Coordinates{Y: 2}, ev.From, "From")
				assert.Equal(t, term.Coordinates{Y: 2, X: 12}, ev.To, "To")
				assert.Equal(t, uri, ev.URI)
				assert.Equal(t, h, ev.Resource)
				assert.Equal(t, "blah\tbleh", ev.Content)
				return false
			}))

		buf.WriteString("blah\tbleh")
		require.True(t, called)
	})

	t.Run("dispatches EventTypeEdit when buffer content is deleted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pub := text.NewPublisher()
		buf, _, cursor := newEdit("aaa\nbbb")

		h := pub.PublishEdit(uri, buf, newMock(ctrl), cursor)
		h.Resize(2, 2)

		var called bool
		pub.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeEdit},
			text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
				called = true
				require.Equal(t, textapi.EventTypeEdit, ev.Type)
				assert.Equal(t, term.Coordinates{}, ev.Start, "Start")
				assert.Equal(t, term.Coordinates{Y: 1}, ev.End, "End")
				assert.Equal(t, term.Coordinates{}, ev.From, "From")
				assert.Equal(t, term.Coordinates{}, ev.To, "To")
				assert.Equal(t, uri, ev.URI)
				assert.Equal(t, h, ev.Resource)
				assert.Equal(t, "", ev.Content)
				return false
			}))

		buf.DeleteLine(term.Coordinates{}, term.Coordinates{})
		require.True(t, called)
	})
}
