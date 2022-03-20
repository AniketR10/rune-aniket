package vi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
)

type mockHandler struct {
	editor.MockHandler
	h viHandlerImpl // used for mode parsing only

	received []term.Event
}

func newMockHandler(buf *cell.Buffer) (ret *mockHandler) {
	ret = new(mockHandler)
	ret.h.init(buf)
	return ret
}

func (h *mockHandler) Resize(width, height int) {
	h.h.Resize(width, height)
}

func (h *mockHandler) Handle(ev term.Event) (bool, bool) {
	h.received = append(h.received, ev)
	switch ev.Ch {
	case '#': // map for convenience
		ev = term.Event{Key: term.KeyEsc}
	case '&':
		ev = term.Event{Key: term.KeyEnter}
	}
	h.h.Handle(ev)
	return false, true
}

func (h *mockHandler) mode() viMode {
	return h.h.mode()
}
func (h *mockHandler) moveToNextLocation(ID string) {
}
func (h *mockHandler) moveToPrevLocation(ID string) {
}
func (h *mockHandler) setLocationList(ID string, l editor.LocationList) {
}
func (h *mockHandler) setCursorAtScroll(pos term.Coordinates) bool {
	return false
}
func (h *mockHandler) cursorAtScroll() term.Coordinates {
	return term.Coordinates{}
}
func (h *mockHandler) subscribeScroll(sub component.ScrollSubscriber) {
}
func (h *mockHandler) newMark(pos term.Coordinates) editor.CursorMark {
	return h.h.newMark(pos)
}
func (h *mockHandler) moveToMark(m editor.CursorMark) {
	h.h.moveToMark(m)
}

func TestViHandle(t *testing.T) {
	tsuite := []struct {
		desc string
		in   string
		want string
	}{
		{
			desc: "delegates events to underlying handler in normal mode",
			in:   "j",
			want: "j",
		},
		{
			desc: "does not propagate normal events upon call to repeat",
			in:   "j.",
			want: "j",
		},
		{
			desc: "repeats update events upon call to repeat",
			in:   "jia#...",
			want: "jia#ia#ia#ia#",
		},
		{
			desc: "repeats insert events upon call to repeat",
			in:   "ia#.io#.",
			want: "ia#ia#io#io#",
		},
		{
			desc: "repeats delete events upon call to repeat",
			in:   "dd.",
			want: "dddd",
		},
		{
			desc: "repeats select + delete events upon call to repeat",
			in:   "jjjvllld..",
			want: "jjjvllldvllldvllld",
		},
		{
			desc: "repeats replace event upon call to repeat",
			in:   "jrl..",
			want: "jrlrlrl",
		},
		{
			desc: "repeats normal update events upon call to repeat",
			in:   ">...",
			want: ">>>>",
		},
		{
			desc: "does no repeat undo",
			in:   ">u...",
			want: ">>>>",
		},
		{
			desc: "does not repeat search events",
			in:   ">/hello#.",
			want: ">/hello#>",
		},
		{
			desc: "repeats select + insert events upon call to repeat",
			in:   "jlvllchello#h.",
			want: "jlvllchello#hvllchello#",
		},
		{
			desc: "repeats delete a word to insert events",
			in:   "jjwcwhello#.",
			want: "jjwcwhello#cwhello#",
		},
		{
			desc: "does not capture combination if there was no update",
			in:   ">i#.",
			want: ">i#>",
		},
		{
			desc: "handles search mode correctly",
			in:   "/put&>i#/put&.",
			want: "/put&>i#/put&>",
		},
		{
			desc: "propagates '.' in search mode",
			in:   "/.&>i#/.&.",
			want: "/.&>i#/.&>",
		},
		{
			desc: "does not repeat select events that did not wind up updating",
			in:   ">jjvlllll#..ihell#.",
			want: ">jjvlllll#>>ihell#ihell#",
		},
	}

	testHandle := func(t *testing.T, in, want string) {
		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := New(buf, "")
		mock := newMockHandler(buf)
		vi.handler = mock
		vi.Resize(100, 100)
		for _, ch := range in {
			ev := term.Event{Ch: ch}
			vi.Handle(ev)
		}
		var received strings.Builder
		for _, ev := range mock.received {
			received.WriteRune(ev.Ch)
		}
		assert.Equal(t, want, received.String())
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			testHandle(t, tcase.in, tcase.want)
		})
	}

	t.Run("state should be cleaned and reset properly on ESC", func(t *testing.T) {
		var in strings.Builder
		var want strings.Builder
		for _, tcase := range tsuite {
			in.WriteString("#") // ESC mapping by mock
			in.WriteString(tcase.in)
			want.WriteString("#")
			want.WriteString(tcase.want)
		}
		testHandle(t, in.String(), want.String())
	})
}

func TestUndo(t *testing.T) {
	const undoFortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`
	suite := []struct {
		name string
		cmd  string
	}{
		{"Insert", "jji\t"},
		{"InsertRowAt", "ji\n"},
		{"DeleteCell", "jjllllx"},
		{"ConflateRow", "J"},
		{"TruncateRowFrom", "jlD"},
		{"TruncateFrom", "lllllldG"},
		{"DeleteRow", "dd"},
		{"Update which effectively replaces", "jjlvlllld"},
		{"Repeat", "jji\t#..."},
		{"DeleteAll", "cGhello\nworld"},
	}

	for _, _tcase := range suite {
		tcase := _tcase
		t.Run(fmt.Sprintf("undo %s", tcase.name), func(t *testing.T) {
			buf := cell.NewBuffer()
			buf.ReadFrom(strings.NewReader(undoFortune))

			vi := New(buf, "")
			vi.Resize(100, 100)

			for i := 0; i < 5; i++ {
				for _, ch := range tcase.cmd {
					ev := term.Event{Ch: ch}
					vi.Handle(ev)
				}
				assert.NotEqual(t, undoFortune, buf.String())
				vi.Handle(term.Event{Key: term.KeyEsc})
				quit, handled := vi.Handle(term.Event{Ch: 'u'})
				assert.False(t, quit)
				assert.True(t, handled)
			}

			assert.Equal(t, undoFortune, buf.String())
		})
	}

	t.Run("undo/redo a series of updates", func(t *testing.T) {
		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(undoFortune))
		prev := buf.String()

		vi := New(buf, "")
		vi.Resize(100, 100)

		for _, tcase := range suite {
			for _, ch := range tcase.cmd {
				ev := term.Event{Ch: ch}
				vi.Handle(ev)
			}
			vi.Handle(term.Event{Key: term.KeyEsc})
		}

		middle := buf.String()

		for range suite {
			quit, handled := vi.Handle(term.Event{Ch: 'u'})
			assert.False(t, quit)
			assert.True(t, handled)
		}

		after := buf.String()
		assert.Equal(t, prev, after)

		for range suite {
			quit, handled := vi.Handle(term.Event{Key: term.KeyCtrlR})
			assert.False(t, quit)
			assert.True(t, handled)
		}

		afterRedo := buf.String()
		assert.Equal(t, middle, afterRedo)

		for range suite {
			quit, handled := vi.Handle(term.Event{Ch: 'u'})
			assert.False(t, quit)
			assert.True(t, handled)
		}

		after = buf.String()
		assert.Equal(t, prev, after)
	})
}
