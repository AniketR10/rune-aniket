package term

import (
	"errors"
	"fmt"
	"strings"
)

// ParseKey parses str into a EventKey Event or returns
// error if it fails to parse it. Note that this function
// is not case sensitive.
func ParseKey(str string) (Event, error) {
	str = strings.ToLower(str)
	switch len(str) {
	case 0:
		return Event{}, errors.New("invalid empty input")
	case 1:
		return Event{Type: EventKey, Ch: []rune(str)[0]}, nil
	default:
		switch str {
		case "<f1>":
			return Event{Type: EventKey, Key: KeyF1}, nil
		case "<f2>":
			return Event{Type: EventKey, Key: KeyF2}, nil
		case "<f3>":
			return Event{Type: EventKey, Key: KeyF3}, nil
		case "<f4>":
			return Event{Type: EventKey, Key: KeyF4}, nil
		case "<f5>":
			return Event{Type: EventKey, Key: KeyF5}, nil
		case "<f6>":
			return Event{Type: EventKey, Key: KeyF6}, nil
		case "<f7>":
			return Event{Type: EventKey, Key: KeyF7}, nil
		case "<f8>":
			return Event{Type: EventKey, Key: KeyF8}, nil
		case "<f9>":
			return Event{Type: EventKey, Key: KeyF9}, nil
		case "<f10>":
			return Event{Type: EventKey, Key: KeyF10}, nil
		case "<f11>":
			return Event{Type: EventKey, Key: KeyF11}, nil
		case "<f12>":
			return Event{Type: EventKey, Key: KeyF12}, nil
		case "<insert>":
			return Event{Type: EventKey, Key: KeyInsert}, nil
		case "<delete>":
			return Event{Type: EventKey, Key: KeyDelete}, nil
		case "<home>":
			return Event{Type: EventKey, Key: KeyHome}, nil
		case "<end>":
			return Event{Type: EventKey, Key: KeyEnd}, nil
		case "<pgup>":
			return Event{Type: EventKey, Key: KeyPgup}, nil
		case "<pgdn>":
			return Event{Type: EventKey, Key: KeyPgdn}, nil
		case "<up>":
			return Event{Type: EventKey, Key: KeyArrowUp}, nil
		case "<down>":
			return Event{Type: EventKey, Key: KeyArrowDown}, nil
		case "<left>":
			return Event{Type: EventKey, Key: KeyArrowLeft}, nil
		case "<right>":
			return Event{Type: EventKey, Key: KeyArrowRight}, nil
		case "<mouse-left>":
			return Event{Type: EventKey, Key: MouseLeft}, nil
		case "<mouse-middle>":
			return Event{Type: EventKey, Key: MouseMiddle}, nil
		case "<mouse-right>":
			return Event{Type: EventKey, Key: MouseRight}, nil
		case "<mouse-release>":
			return Event{Type: EventKey, Key: MouseRelease}, nil
		case "<mouse-wheel-up>":
			return Event{Type: EventKey, Key: MouseWheelUp}, nil
		case "<mouse-wheel-down>":
			return Event{Type: EventKey, Key: MouseWheelDown}, nil
		case "<c-`>", "<ctrl-`>":
			return Event{Type: EventKey, Key: KeyCtrlTilde}, nil
		case "<c-2>", "<ctrl-2>":
			return Event{Type: EventKey, Key: KeyCtrl2}, nil
		case "<c-space>", "<ctrl-space>":
			return Event{Type: EventKey, Key: KeyCtrlSpace}, nil
		case "<c-a>", "<ctrl-a>":
			return Event{Type: EventKey, Key: KeyCtrlA}, nil
		case "<c-b>", "<ctrl-b>":
			return Event{Type: EventKey, Key: KeyCtrlB}, nil
		case "<c-c>", "<ctrl-c>":
			return Event{Type: EventKey, Key: KeyCtrlC}, nil
		case "<c-d>", "<ctrl-d>":
			return Event{Type: EventKey, Key: KeyCtrlD}, nil
		case "<c-e>", "<ctrl-e>":
			return Event{Type: EventKey, Key: KeyCtrlE}, nil
		case "<c-f>", "<ctrl-f>":
			return Event{Type: EventKey, Key: KeyCtrlF}, nil
		case "<c-g>", "<ctrl-g>":
			return Event{Type: EventKey, Key: KeyCtrlG}, nil
		case "<c-backspace>", "<ctrl-backspace>":
			return Event{Type: EventKey, Key: KeyBackspace}, nil
		case "<c-h>", "<ctrl-h>":
			return Event{Type: EventKey, Key: KeyCtrlH}, nil
		case "<c-tab>", "<ctrl-tab>":
			return Event{Type: EventKey, Key: KeyTab}, nil
		case "<c-i>", "<ctrl-i>":
			return Event{Type: EventKey, Key: KeyCtrlI}, nil
		case "<c-j>", "<ctrl-j>":
			return Event{Type: EventKey, Key: KeyCtrlJ}, nil
		case "<c-k>", "<ctrl-k>":
			return Event{Type: EventKey, Key: KeyCtrlK}, nil
		case "<c-l>", "<ctrl-l>":
			return Event{Type: EventKey, Key: KeyCtrlL}, nil
		case "<enter>":
			return Event{Type: EventKey, Key: KeyEnter}, nil
		case "<c-m>", "<ctrl-m>":
			return Event{Type: EventKey, Key: KeyCtrlM}, nil
		case "<c-n>", "<ctrl-n>":
			return Event{Type: EventKey, Key: KeyCtrlN}, nil
		case "<c-o>", "<ctrl-o>":
			return Event{Type: EventKey, Key: KeyCtrlO}, nil
		case "<c-p>", "<ctrl-p>":
			return Event{Type: EventKey, Key: KeyCtrlP}, nil
		case "<c-q>", "<ctrl-q>":
			return Event{Type: EventKey, Key: KeyCtrlQ}, nil
		case "<c-r>", "<ctrl-r>":
			return Event{Type: EventKey, Key: KeyCtrlR}, nil
		case "<c-s>", "<ctrl-s>":
			return Event{Type: EventKey, Key: KeyCtrlS}, nil
		case "<c-t>", "<ctrl-t>":
			return Event{Type: EventKey, Key: KeyCtrlT}, nil
		case "<c-u>", "<ctrl-u>":
			return Event{Type: EventKey, Key: KeyCtrlU}, nil
		case "<c-v>", "<ctrl-v>":
			return Event{Type: EventKey, Key: KeyCtrlV}, nil
		case "<c-w>", "<ctrl-w>":
			return Event{Type: EventKey, Key: KeyCtrlW}, nil
		case "<c-x>", "<ctrl-x>":
			return Event{Type: EventKey, Key: KeyCtrlX}, nil
		case "<c-y>", "<ctrl-y>":
			return Event{Type: EventKey, Key: KeyCtrlY}, nil
		case "<c-z>", "<ctrl-z>":
			return Event{Type: EventKey, Key: KeyCtrlZ}, nil
		case "<esc>":
			return Event{Type: EventKey, Key: KeyEsc}, nil
		case "<c-[>", "<ctrl-[>":
			return Event{Type: EventKey, Key: KeyCtrlLsqBracket}, nil
		case "<c-3>", "<ctrl-3>":
			return Event{Type: EventKey, Key: KeyCtrl3}, nil
		case "<c-4>", "<ctrl-4>":
			return Event{Type: EventKey, Key: KeyCtrl4}, nil
		case "<c-\\>", "<ctrl-\\>":
			return Event{Type: EventKey, Key: KeyCtrlBackslash}, nil
		case "<c-5>", "<ctrl-5>":
			return Event{Type: EventKey, Key: KeyCtrl5}, nil
		case "<c-]>", "<ctrl-]>":
			return Event{Type: EventKey, Key: KeyCtrlRsqBracket}, nil
		case "<c-6>", "<ctrl-6>":
			return Event{Type: EventKey, Key: KeyCtrl6}, nil
		case "<c-7>", "<ctrl-7>":
			return Event{Type: EventKey, Key: KeyCtrl7}, nil
		case "<c-/>", "<ctrl-/>":
			return Event{Type: EventKey, Key: KeyCtrlSlash}, nil
		case "<c-_>", "<ctrl-_>":
			return Event{Type: EventKey, Key: KeyCtrlUnderscore}, nil
		case "<space>":
			return Event{Type: EventKey, Key: KeySpace}, nil
		case "<backspace>":
			return Event{Type: EventKey, Key: KeyBackspace2}, nil
		case "<c-8>", "<ctrl-8>":
			return Event{Type: EventKey, Key: KeyCtrl8}, nil
		case "<m-f1>":
			return Event{Type: EventKey, Key: KeyF1, Mod: ModAlt}, nil
		case "<m-f2>":
			return Event{Type: EventKey, Key: KeyF2, Mod: ModAlt}, nil
		case "<m-f3>":
			return Event{Type: EventKey, Key: KeyF3, Mod: ModAlt}, nil
		case "<m-f4>":
			return Event{Type: EventKey, Key: KeyF4, Mod: ModAlt}, nil
		case "<m-f5>":
			return Event{Type: EventKey, Key: KeyF5, Mod: ModAlt}, nil
		case "<m-f6>":
			return Event{Type: EventKey, Key: KeyF6, Mod: ModAlt}, nil
		case "<m-f7>":
			return Event{Type: EventKey, Key: KeyF7, Mod: ModAlt}, nil
		case "<m-f8>":
			return Event{Type: EventKey, Key: KeyF8, Mod: ModAlt}, nil
		case "<m-f9>":
			return Event{Type: EventKey, Key: KeyF9, Mod: ModAlt}, nil
		case "<m-f10>":
			return Event{Type: EventKey, Key: KeyF10, Mod: ModAlt}, nil
		case "<m-f11>":
			return Event{Type: EventKey, Key: KeyF11, Mod: ModAlt}, nil
		case "<m-f12>":
			return Event{Type: EventKey, Key: KeyF12, Mod: ModAlt}, nil
		case "<m-insert>":
			return Event{Type: EventKey, Key: KeyInsert, Mod: ModAlt}, nil
		case "<m-delete>":
			return Event{Type: EventKey, Key: KeyDelete, Mod: ModAlt}, nil
		case "<m-home>":
			return Event{Type: EventKey, Key: KeyHome, Mod: ModAlt}, nil
		case "<m-end>":
			return Event{Type: EventKey, Key: KeyEnd, Mod: ModAlt}, nil
		case "<m-pgup>":
			return Event{Type: EventKey, Key: KeyPgup, Mod: ModAlt}, nil
		case "<m-pgdn>":
			return Event{Type: EventKey, Key: KeyPgdn, Mod: ModAlt}, nil
		case "<m-up>":
			return Event{Type: EventKey, Key: KeyArrowUp, Mod: ModAlt}, nil
		case "<m-down>":
			return Event{Type: EventKey, Key: KeyArrowDown, Mod: ModAlt}, nil
		case "<m-left>":
			return Event{Type: EventKey, Key: KeyArrowLeft, Mod: ModAlt}, nil
		case "<m-right>":
			return Event{Type: EventKey, Key: KeyArrowRight, Mod: ModAlt}, nil
		case "<m-mouse-left>":
			return Event{Type: EventKey, Key: MouseLeft, Mod: ModAlt}, nil
		case "<m-mouse-middle>":
			return Event{Type: EventKey, Key: MouseMiddle, Mod: ModAlt}, nil
		case "<m-mouse-right>":
			return Event{Type: EventKey, Key: MouseRight, Mod: ModAlt}, nil
		case "<m-mouse-release>":
			return Event{Type: EventKey, Key: MouseRelease, Mod: ModAlt}, nil
		case "<m-mouse-wheel-up>":
			return Event{Type: EventKey, Key: MouseWheelUp, Mod: ModAlt}, nil
		case "<m-mouse-wheel-down>":
			return Event{Type: EventKey, Key: MouseWheelDown, Mod: ModAlt}, nil
		case "<m-c-`>", "<c-m-`>", "<mod-ctrl-`>", "<ctrl-mod-`>":
			return Event{Type: EventKey, Key: KeyCtrlTilde, Mod: ModAlt}, nil
		case "<m-c-2>", "<c-m-2>", "<mod-ctrl-2>", "<ctrl-mod-2>":
			return Event{Type: EventKey, Key: KeyCtrl2, Mod: ModAlt}, nil
		case "<m-c-space>", "<c-m-space>", "<mod-ctrl-space>", "<ctrl-mod-space>":
			return Event{Type: EventKey, Key: KeyCtrlSpace, Mod: ModAlt}, nil
		case "<m-c-a>", "<c-m-a>", "<mod-ctrl-a>", "<ctrl-mod-a>":
			return Event{Type: EventKey, Key: KeyCtrlA, Mod: ModAlt}, nil
		case "<m-c-b>", "<c-m-b>", "<mod-ctrl-b>", "<ctrl-mod-b>":
			return Event{Type: EventKey, Key: KeyCtrlB, Mod: ModAlt}, nil
		case "<m-c-c>", "<c-m-c>", "<mod-ctrl-c>", "<ctrl-mod-c>":
			return Event{Type: EventKey, Key: KeyCtrlC, Mod: ModAlt}, nil
		case "<m-c-d>", "<c-m-d>", "<mod-ctrl-d>", "<ctrl-mod-d>":
			return Event{Type: EventKey, Key: KeyCtrlD, Mod: ModAlt}, nil
		case "<m-c-e>", "<c-m-e>", "<mod-ctrl-e>", "<ctrl-mod-e>":
			return Event{Type: EventKey, Key: KeyCtrlE, Mod: ModAlt}, nil
		case "<m-c-f>", "<c-m-f>", "<mod-ctrl-f>", "<ctrl-mod-f>":
			return Event{Type: EventKey, Key: KeyCtrlF, Mod: ModAlt}, nil
		case "<m-c-g>", "<c-m-g>", "<mod-ctrl-g>", "<ctrl-mod-g>":
			return Event{Type: EventKey, Key: KeyCtrlG, Mod: ModAlt}, nil
		case "<m-c-backspace>", "<c-m-backspace>", "<mod-ctrl-backspace>", "<ctrl-mod-backspace>":
			return Event{Type: EventKey, Key: KeyBackspace, Mod: ModAlt}, nil
		case "<m-c-h>", "<c-m-h>", "<mod-ctrl-h>", "<ctrl-mod-h>":
			return Event{Type: EventKey, Key: KeyCtrlH, Mod: ModAlt}, nil
		case "<m-c-tab>", "<c-m-tab>", "<mod-ctrl-tab>", "<ctrl-mod-tab>":
			return Event{Type: EventKey, Key: KeyTab, Mod: ModAlt}, nil
		case "<m-c-i>", "<c-m-i>", "<mod-ctrl-i>", "<ctrl-mod-i>":
			return Event{Type: EventKey, Key: KeyCtrlI, Mod: ModAlt}, nil
		case "<m-c-j>", "<c-m-j>", "<mod-ctrl-j>", "<ctrl-mod-j>":
			return Event{Type: EventKey, Key: KeyCtrlJ, Mod: ModAlt}, nil
		case "<m-c-k>", "<c-m-k>", "<mod-ctrl-k>", "<ctrl-mod-k>":
			return Event{Type: EventKey, Key: KeyCtrlK, Mod: ModAlt}, nil
		case "<m-c-l>", "<c-m-l>", "<mod-ctrl-l>", "<ctrl-mod-l>":
			return Event{Type: EventKey, Key: KeyCtrlL, Mod: ModAlt}, nil
		case "<m-enter>":
			return Event{Type: EventKey, Key: KeyEnter, Mod: ModAlt}, nil
		case "<m-c-m>", "<c-m-m>", "<mod-ctrl-m>", "<ctrl-mod-m>":
			return Event{Type: EventKey, Key: KeyCtrlM, Mod: ModAlt}, nil
		case "<m-c-n>", "<c-m-n>", "<mod-ctrl-n>", "<ctrl-mod-n>":
			return Event{Type: EventKey, Key: KeyCtrlN, Mod: ModAlt}, nil
		case "<m-c-o>", "<c-m-o>", "<mod-ctrl-o>", "<ctrl-mod-o>":
			return Event{Type: EventKey, Key: KeyCtrlO, Mod: ModAlt}, nil
		case "<m-c-p>", "<c-m-p>", "<mod-ctrl-p>", "<ctrl-mod-p>":
			return Event{Type: EventKey, Key: KeyCtrlP, Mod: ModAlt}, nil
		case "<m-c-q>", "<c-m-q>", "<mod-ctrl-q>", "<ctrl-mod-q>":
			return Event{Type: EventKey, Key: KeyCtrlQ, Mod: ModAlt}, nil
		case "<m-c-r>", "<c-m-r>", "<mod-ctrl-r>", "<ctrl-mod-r>":
			return Event{Type: EventKey, Key: KeyCtrlR, Mod: ModAlt}, nil
		case "<m-c-s>", "<c-m-s>", "<mod-ctrl-s>", "<ctrl-mod-s>":
			return Event{Type: EventKey, Key: KeyCtrlS, Mod: ModAlt}, nil
		case "<m-c-t>", "<c-m-t>", "<mod-ctrl-t>", "<ctrl-mod-t>":
			return Event{Type: EventKey, Key: KeyCtrlT, Mod: ModAlt}, nil
		case "<m-c-u>", "<c-m-u>", "<mod-ctrl-u>", "<ctrl-mod-u>":
			return Event{Type: EventKey, Key: KeyCtrlU, Mod: ModAlt}, nil
		case "<m-c-v>", "<c-m-v>", "<mod-ctrl-v>", "<ctrl-mod-v>":
			return Event{Type: EventKey, Key: KeyCtrlV, Mod: ModAlt}, nil
		case "<m-c-w>", "<c-m-w>", "<mod-ctrl-w>", "<ctrl-mod-w>":
			return Event{Type: EventKey, Key: KeyCtrlW, Mod: ModAlt}, nil
		case "<m-c-x>", "<c-m-x>", "<mod-ctrl-x>", "<ctrl-mod-x>":
			return Event{Type: EventKey, Key: KeyCtrlX, Mod: ModAlt}, nil
		case "<m-c-y>", "<c-m-y>", "<mod-ctrl-y>", "<ctrl-mod-y>":
			return Event{Type: EventKey, Key: KeyCtrlY, Mod: ModAlt}, nil
		case "<m-c-z>", "<c-m-z>", "<mod-ctrl-z>", "<ctrl-mod-z>":
			return Event{Type: EventKey, Key: KeyCtrlZ, Mod: ModAlt}, nil
		case "<m-esc>":
			return Event{Type: EventKey, Key: KeyEsc, Mod: ModAlt}, nil
		case "<m-c-[>", "<c-m-[>", "<mod-ctrl-[>", "<ctrl-mod-[>":
			return Event{Type: EventKey, Key: KeyCtrlLsqBracket, Mod: ModAlt}, nil
		case "<m-c-3>", "<c-m-3>", "<mod-ctrl-3>", "<ctrl-mod-3>":
			return Event{Type: EventKey, Key: KeyCtrl3, Mod: ModAlt}, nil
		case "<m-c-4>", "<c-m-4>", "<mod-ctrl-4>", "<ctrl-mod-4>":
			return Event{Type: EventKey, Key: KeyCtrl4, Mod: ModAlt}, nil
		case "<m-c-\\>", "<c-m-\\>", "<mod-ctrl-\\>", "<ctrl-mod-\\>":
			return Event{Type: EventKey, Key: KeyCtrlBackslash, Mod: ModAlt}, nil
		case "<m-c-5>", "<c-m-5>", "<mod-ctrl-5>", "<ctrl-mod-5>":
			return Event{Type: EventKey, Key: KeyCtrl5, Mod: ModAlt}, nil
		case "<m-c-]>", "<c-m-]>", "<mod-ctrl-]>", "<ctrl-mod-]>":
			return Event{Type: EventKey, Key: KeyCtrlRsqBracket, Mod: ModAlt}, nil
		case "<m-c-6>", "<c-m-6>", "<mod-ctrl-6>", "<ctrl-mod-6>":
			return Event{Type: EventKey, Key: KeyCtrl6, Mod: ModAlt}, nil
		case "<m-c-7>", "<c-m-7>", "<mod-ctrl-7>", "<ctrl-mod-7>":
			return Event{Type: EventKey, Key: KeyCtrl7, Mod: ModAlt}, nil
		case "<m-c-/>", "<c-m-/>", "<mod-ctrl-/>", "<ctrl-mod-/>":
			return Event{Type: EventKey, Key: KeyCtrlSlash, Mod: ModAlt}, nil
		case "<m-c-_>", "<c-m-_>", "<mod-ctrl-_>", "<ctrl-mod-_>":
			return Event{Type: EventKey, Key: KeyCtrlUnderscore, Mod: ModAlt}, nil
		case "<m-space>":
			return Event{Type: EventKey, Key: KeySpace, Mod: ModAlt}, nil
		case "<m-backspace>":
			return Event{Type: EventKey, Key: KeyBackspace2, Mod: ModAlt}, nil
		case "<m-c-8>", "<c-m-8>", "<mod-ctrl-8>", "<ctrl-mod-8>":
			return Event{Type: EventKey, Key: KeyCtrl8, Mod: ModAlt}, nil
		default:
			return Event{}, fmt.Errorf("invalid key: '%s'", str)
		}
	}
}
