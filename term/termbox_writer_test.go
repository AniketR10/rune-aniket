package term

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/tcell/v3/termbox"
)

// TODO
func TestTermboxEventConvert(t *testing.T) {
	t.Run("fully reversible key combinations", func(t *testing.T) {
		suite := []struct {
			ev  KeyComb
			tev termbox.Event
		}{
			{KeyComb{Ch: 'a'}, termbox.Event{Ch: 'a'}},
			{KeyComb{Ch: 'A'}, termbox.Event{Ch: 'A'}},
			{KeyComb{Ch: '1'}, termbox.Event{Ch: '1'}},
			{KeyComb{Ch: '!'}, termbox.Event{Ch: '!'}},
			{KeyComb{Ch: '_'}, termbox.Event{Ch: '_'}},
			{KeyComb{Ch: '-'}, termbox.Event{Ch: '-'}},
			{KeyComb{Ch: '>'}, termbox.Event{Ch: '>'}},
			{KeyComb{Ch: '+'}, termbox.Event{Ch: '+'}},
			{KeyComb{Ch: '.'}, termbox.Event{Ch: '.'}},
			{KeyComb{Ch: ' '}, termbox.Event{Ch: ' '}},
			{KeyComb{Ch: '`'}, termbox.Event{Ch: '`'}},
			{KeyComb{Ch: '\t'}, termbox.Event{Ch: '\t'}},
			{KeyComb{Ch: ' ', Key: KeySpace}, termbox.Event{Ch: ' ', Key: termbox.KeySpace}},
			{KeyComb{Key: KeyF1}, termbox.Event{Key: termbox.KeyF1}},
			{KeyComb{Key: KeyF2}, termbox.Event{Key: termbox.KeyF2}},
			{KeyComb{Key: KeyF3}, termbox.Event{Key: termbox.KeyF3}},
			{KeyComb{Key: KeyF4}, termbox.Event{Key: termbox.KeyF4}},
			{KeyComb{Key: KeyF5}, termbox.Event{Key: termbox.KeyF5}},
			{KeyComb{Key: KeyF6}, termbox.Event{Key: termbox.KeyF6}},
			{KeyComb{Key: KeyF7}, termbox.Event{Key: termbox.KeyF7}},
			{KeyComb{Key: KeyF8}, termbox.Event{Key: termbox.KeyF8}},
			{KeyComb{Key: KeyF9}, termbox.Event{Key: termbox.KeyF9}},
			{KeyComb{Key: KeyF10}, termbox.Event{Key: termbox.KeyF10}},
			{KeyComb{Key: KeyF11}, termbox.Event{Key: termbox.KeyF11}},
			{KeyComb{Key: KeyF12}, termbox.Event{Key: termbox.KeyF12}},
			{KeyComb{Key: KeyInsert}, termbox.Event{Key: termbox.KeyInsert}},
			{KeyComb{Key: KeyDelete}, termbox.Event{Key: termbox.KeyDelete}},
			{KeyComb{Key: KeyHome}, termbox.Event{Key: termbox.KeyHome}},
			{KeyComb{Key: KeyEnd}, termbox.Event{Key: termbox.KeyEnd}},
			{KeyComb{Key: KeyArrowUp}, termbox.Event{Key: termbox.KeyArrowUp}},
			{KeyComb{Key: KeyArrowDown}, termbox.Event{Key: termbox.KeyArrowDown}},
			{KeyComb{Key: KeyArrowRight}, termbox.Event{Key: termbox.KeyArrowRight}},
			{KeyComb{Key: KeyArrowLeft}, termbox.Event{Key: termbox.KeyArrowLeft}},
			{KeyComb{Mod: ModCtrl, Ch: 'a'}, termbox.Event{Key: termbox.KeyCtrlA}},
			{KeyComb{Mod: ModCtrl, Ch: 'b'}, termbox.Event{Key: termbox.KeyCtrlB}},
			{KeyComb{Mod: ModCtrl, Ch: 'c'}, termbox.Event{Key: termbox.KeyCtrlC}},
			{KeyComb{Mod: ModCtrl, Ch: 'd'}, termbox.Event{Key: termbox.KeyCtrlD}},
			{KeyComb{Mod: ModCtrl, Ch: 'e'}, termbox.Event{Key: termbox.KeyCtrlE}},
			{KeyComb{Mod: ModCtrl, Ch: 'f'}, termbox.Event{Key: termbox.KeyCtrlF}},
			{KeyComb{Mod: ModCtrl, Ch: 'g'}, termbox.Event{Key: termbox.KeyCtrlG}},
			{KeyComb{Mod: ModCtrl, Ch: 'j'}, termbox.Event{Key: termbox.KeyCtrlJ}},
			{KeyComb{Mod: ModCtrl, Ch: 'k'}, termbox.Event{Key: termbox.KeyCtrlK}},
			{KeyComb{Mod: ModCtrl, Ch: 'l'}, termbox.Event{Key: termbox.KeyCtrlL}},
			{KeyComb{Mod: ModCtrl, Ch: 'n'}, termbox.Event{Key: termbox.KeyCtrlN}},
			{KeyComb{Mod: ModCtrl, Ch: 'o'}, termbox.Event{Key: termbox.KeyCtrlO}},
			{KeyComb{Mod: ModCtrl, Ch: 'p'}, termbox.Event{Key: termbox.KeyCtrlP}},
			{KeyComb{Mod: ModCtrl, Ch: 'q'}, termbox.Event{Key: termbox.KeyCtrlQ}},
			{KeyComb{Mod: ModCtrl, Ch: 'r'}, termbox.Event{Key: termbox.KeyCtrlR}},
			{KeyComb{Mod: ModCtrl, Ch: 's'}, termbox.Event{Key: termbox.KeyCtrlS}},
			{KeyComb{Mod: ModCtrl, Ch: 't'}, termbox.Event{Key: termbox.KeyCtrlT}},
			{KeyComb{Mod: ModCtrl, Ch: 'u'}, termbox.Event{Key: termbox.KeyCtrlU}},
			{KeyComb{Mod: ModCtrl, Ch: 'v'}, termbox.Event{Key: termbox.KeyCtrlV}},
			{KeyComb{Mod: ModCtrl, Ch: 'w'}, termbox.Event{Key: termbox.KeyCtrlW}},
			{KeyComb{Mod: ModCtrl, Ch: 'x'}, termbox.Event{Key: termbox.KeyCtrlX}},
			{KeyComb{Mod: ModCtrl, Ch: 'y'}, termbox.Event{Key: termbox.KeyCtrlY}},
			{KeyComb{Mod: ModCtrl, Ch: 'z'}, termbox.Event{Key: termbox.KeyCtrlZ}},
			{KeyComb{Key: KeyBackspace}, termbox.Event{Key: termbox.KeyBackspace2}},
			{KeyComb{Key: KeyTab}, termbox.Event{Key: termbox.KeyTab}},
			{KeyComb{Key: KeyEnter}, termbox.Event{Key: termbox.KeyEnter}},
			{KeyComb{Key: KeyEsc}, termbox.Event{Key: termbox.KeyEsc}},
			{KeyComb{Key: KeyPgdn}, termbox.Event{Key: termbox.KeyPgdn}},
			{KeyComb{Key: KeyPgup}, termbox.Event{Key: termbox.KeyPgup}},
			{KeyComb{Key: KeySpace}, termbox.Event{Key: termbox.KeySpace}},
			{KeyComb{Key: KeySpace, Mod: ModCtrl}, termbox.Event{Key: termbox.KeyCtrlSpace}},
			{KeyComb{Key: MouseLeft}, termbox.Event{Key: termbox.MouseLeft}},
			{KeyComb{Key: MouseRight}, termbox.Event{Key: termbox.MouseRight}},
			{KeyComb{Key: MouseMiddle}, termbox.Event{Key: termbox.MouseMiddle}},
			{KeyComb{Key: MouseRelease}, termbox.Event{Key: termbox.MouseRelease}},
			{KeyComb{Key: MouseWheelUp}, termbox.Event{Key: termbox.MouseWheelUp}},
			{KeyComb{Key: MouseWheelDown}, termbox.Event{Key: termbox.MouseWheelDown}},

			// ambiguous keys
			{KeyComb{Ch: 'h', Mod: ModCtrl}, termbox.Event{Key: termbox.KeyCtrlH}},
			{KeyComb{Mod: ModCtrl, Key: KeySpace}, termbox.Event{Key: termbox.KeyCtrl2}},
			{KeyComb{Key: KeyEsc}, termbox.Event{Key: termbox.KeyCtrl3}},
			{KeyComb{Ch: '6', Mod: ModCtrl}, termbox.Event{Key: termbox.KeyCtrl6}},
			{KeyComb{Mod: ModCtrl, Ch: '/'}, termbox.Event{Key: termbox.KeyCtrl7}},
			{KeyComb{Ch: '/', Mod: ModCtrl}, termbox.Event{Key: termbox.KeyCtrlSlash}},
			{KeyComb{Ch: '\\', Mod: ModCtrl}, termbox.Event{Key: termbox.KeyCtrlBackslash}},
			{KeyComb{Key: KeyEsc}, termbox.Event{Key: termbox.KeyCtrlLsqBracket}},
			{KeyComb{Ch: '\\', Mod: ModCtrl}, termbox.Event{Key: termbox.KeyCtrl4}},
			{KeyComb{Ch: ']', Mod: ModCtrl}, termbox.Event{Key: termbox.KeyCtrl5}},
			{KeyComb{Mod: ModCtrl, Ch: '/'}, termbox.Event{Key: termbox.KeyCtrlUnderscore}},
			{KeyComb{Key: KeyBackspace}, termbox.Event{Key: termbox.KeyCtrl8}},
		}

		for i, test := range suite {
			t.Run(fmt.Sprintf("event to termbox event first: %d", i), func(t *testing.T) {
				ev := Event{Type: EventKey, Ch: test.ev.Ch, Key: test.ev.Key, Mod: test.ev.Mod}
				actualTev := eventToTermboxEvent(ev)
				test.tev.Type = termbox.EventKey // make test cases easier to spell out
				require.Equal(t, test.tev, actualTev, "event to termbox event: %+v: %+v",
					ev, actualTev)

				actualEv := termboxEventToEvent(actualTev)
				assert.Equal(t, ev, actualEv, "termbox event to event: %+v: %+v",
					actualTev, actualEv)
			})

			t.Run(fmt.Sprintf("termbox event to event: %d", i), func(t *testing.T) {
				test.tev.Type = termbox.EventKey
				actualEv := termboxEventToEvent(test.tev)
				ev := Event{Type: EventKey, Ch: test.ev.Ch, Key: test.ev.Key, Mod: test.ev.Mod}
				assert.Equal(t, ev, actualEv, "termbox event to event: %+v: %+v",
					test.tev, actualEv)

				actualTev := eventToTermboxEvent(actualEv)
				require.Equal(t, test.tev, actualTev, "event to termbox event: %+v: %+v",
					ev, actualTev)
			})
		}
	})

	t.Run("not fully reversible, but compatible combinations", func(t *testing.T) {
		suite := []struct {
			ev  KeyComb
			tev termbox.Event
		}{
			{KeyComb{Ch: '~'}, termbox.Event{Key: termbox.KeyTilde}},
		}
		for i, test := range suite {
			t.Run(fmt.Sprintf("termbox event to event: %d", i), func(t *testing.T) {
				test.tev.Type = termbox.EventKey
				actualEv := termboxEventToEvent(test.tev)
				ev := Event{Type: EventKey, Ch: test.ev.Ch, Key: test.ev.Key, Mod: test.ev.Mod}
				assert.Equal(t, ev, actualEv, "termbox event to event: %+v: %+v",
					test.tev, actualEv)
			})
		}
	})
}
