package text

import (
	"context"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEdit(content string) (*cell.Buffer, *component.Scroll, *Cursor) {
	buf := cell.NewBuffer()
	buf.WriteString(content)
	scroll := component.NewScroll()
	scroll.InitWithBuffer(buf)
	cursor := NewCursor(scroll)
	return buf, scroll, cursor
}

func newMock(ctrl *gomock.Controller) *MockHandler {
	mock := NewMockHandler(ctrl)
	mock.EXPECT().Resize(gomock.Any(), gomock.Any()).AnyTimes()
	return mock
}

func TestPublisher(t *testing.T) {
	t.Run("publishes EventTypeOpen when PublishEdit is called", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		pub := NewPublisher()
		content := "1. She interviews"
		name := "Teamshares"

		var eventHandler Handler
		pub.SubscribeEditorEvents([]EventType{EventTypeOpen},
			FuncEventHandler(func(ctx context.Context, ev Event) bool {
				assert.Equal(t, EventTypeOpen, ev.Type)
				assert.Equal(t, content, ev.Content)
				assert.Equal(t, name, ev.ResourceName)
				eventHandler = ev.Resource
				return false
			}))

		buf, _, cursor := newEdit(content)
		returnedHandler := pub.PublishEdit(name, buf, newMock(ctrl), cursor)
		assert.Equal(t, eventHandler, returnedHandler)
	})

	t.Run("publishes EventTypeFocus when PublishEdit is called", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		pub := NewPublisher()
		content := "2. She gets hired"
		name := "Teamshares"

		var eventHandler Handler
		pub.SubscribeEditorEvents([]EventType{EventTypeFocus},
			FuncEventHandler(func(ctx context.Context, ev Event) bool {
				assert.Equal(t, EventTypeFocus, ev.Type)
				assert.Equal(t, name, ev.ResourceName)
				eventHandler = ev.Resource
				return false
			}))

		buf, _, cursor := newEdit(content)
		returnedHandler := pub.PublishEdit(name, buf, newMock(ctrl), cursor)
		assert.Equal(t, eventHandler, returnedHandler)
	})

	t.Run("subscribes to multiple events", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		pub := NewPublisher()
		content := "3. SHE GOT HIRED, I KNEW IT!!!"
		name := "Teamshares"

		var eventHandler Handler
		var fired int
		pub.SubscribeEditorEvents([]EventType{EventTypeOpen, EventTypeFocus},
			FuncEventHandler(func(ctx context.Context, ev Event) bool {
				fired++
				assert.Equal(t, name, ev.ResourceName)
				eventHandler = ev.Resource
				return false
			}))

		buf, _, cursor := newEdit(content)
		returnedHandler := pub.PublishEdit(name, buf, newMock(ctrl), cursor)
		assert.Equal(t, eventHandler, returnedHandler)
		assert.Equal(t, 2, fired)
	})

	t.Run("dispatches EventTypeCursor when given cursor changes on Handle", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		pub := NewPublisher()
		mock := newMock(ctrl)
		buf, _, cursor := newEdit("\n\n")

		h := pub.PublishEdit("zsh", buf, mock, cursor)
		h.Resize(2, 2)

		var called bool
		pub.SubscribeEditorEvents([]EventType{EventTypeCursor},
			FuncEventHandler(func(ctx context.Context, ev Event) bool {
				require.Equal(t, EventTypeCursor, ev.Type)
				assert.Equal(t, ev.ResourceName, "zsh")
				assert.Equal(t, ev.Resource, h)
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
		pub := NewPublisher()
		buf, scroll, cursor := newEdit("\n\n")

		h := pub.PublishEdit("zsh", buf, newMock(ctrl), cursor)
		h.Resize(2, 2)
		require.True(t, scroll.SeekDown())

		at := term.Coordinates{X: -1}
		pub.SubscribeEditorEvents([]EventType{EventTypeScroll},
			FuncEventHandler(func(ctx context.Context, ev Event) bool {
				require.Equal(t, EventTypeScroll, ev.Type)
				assert.Equal(t, "zsh", ev.ResourceName)
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

	t.Run("dispatches EventTypeUpdate when buffer content is inserted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pub := NewPublisher()
		buf, _, cursor := newEdit("\n\n")

		h := pub.PublishEdit("zsh", buf, newMock(ctrl), cursor)
		h.Resize(2, 2)

		var called bool
		pub.SubscribeEditorEvents([]EventType{EventTypeUpdate},
			FuncEventHandler(func(ctx context.Context, ev Event) bool {
				called = true
				require.Equal(t, EventTypeUpdate, ev.Type)
				assert.Equal(t, term.Coordinates{Y: 2}, ev.Start, "Start")
				assert.Equal(t, term.Coordinates{Y: 2}, ev.End, "End")
				assert.Equal(t, term.Coordinates{Y: 2}, ev.From, "From")
				assert.Equal(t, term.Coordinates{Y: 2, X: 12}, ev.To, "To")
				assert.Equal(t, "zsh", ev.ResourceName)
				assert.Equal(t, h, ev.Resource)
				assert.Equal(t, "blah\tbleh", ev.Content)
				return false
			}))

		buf.WriteString("blah\tbleh")
		require.True(t, called)
	})

	t.Run("dispatches EventTypeUpdate when buffer content is deleted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pub := NewPublisher()
		buf, _, cursor := newEdit("aaa\nbbb")

		h := pub.PublishEdit("zsh", buf, newMock(ctrl), cursor)
		h.Resize(2, 2)

		var called bool
		pub.SubscribeEditorEvents([]EventType{EventTypeUpdate},
			FuncEventHandler(func(ctx context.Context, ev Event) bool {
				called = true
				require.Equal(t, EventTypeUpdate, ev.Type)
				assert.Equal(t, term.Coordinates{}, ev.Start, "Start")
				assert.Equal(t, term.Coordinates{Y: 1}, ev.End, "End")
				assert.Equal(t, term.Coordinates{}, ev.From, "From")
				assert.Equal(t, term.Coordinates{}, ev.To, "To")
				assert.Equal(t, "zsh", ev.ResourceName)
				assert.Equal(t, h, ev.Resource)
				assert.Equal(t, "", ev.Content)
				return false
			}))

		buf.DeleteLine(term.Coordinates{}, term.Coordinates{})
		require.True(t, called)
	})
}
