package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestDrawAttributes(t *testing.T) {
	t.Run("draws cells correctly and respecting bounds", func(t *testing.T) {
		w := term.NewStringWriter(9, 5)
		u := &TestComponent{Ch: 'X'}
		s := WithAttrSetter(u)

		s.Resize(8, 4)

		tests := []testutil.ComponentTestCase{
			{
				nil, `
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
         `,
			},
		}

		testutil.TestComponent(t, s, w, tests)
	})
}

func TestAttrSetter(t *testing.T) {
	t.Run("attr oob is ignored", func(t *testing.T) {
		w := term.NewStringWriter(2, 2)
		s := WithAttrSetter(&TestComponent{Ch: 'x'})
		s.SetAttrAt(term.Coordinates{X: 2, Y: 2}, term.Attributes{Fg: term.ColorRed})

		s.Resize(2, 2)

		// test that it doesn't panic
		s.Draw(w)
	})

	t.Run("happy path", func(t *testing.T) {
		w := cell.NewBufferWriter(4, 4)
		s := WithAttrSetter(&TestComponent{Ch: 'a'})
		s.SetAttr(term.Attributes{Fg: term.ColorBlue, Bg: term.ColorCyan})
		s.SetAttrAt(term.Coordinates{X: 3, Y: 3},
			term.Attributes{Fg: term.ColorRed | term.AttrBold, Bg: term.ColorGreen | term.AttrUnderline})

		s.Resize(4, 4)
		s.Draw(w)

		for y, row := range w.RawCells() {
			for x, cell := range row {
				if y == 3 && x == 3 {
					assert.True(t, (term.ColorGreen|term.AttrUnderline)&cell.Bg != 0)
					assert.True(t, (term.ColorRed|term.AttrBold)&cell.Fg != 0)
				} else {
					assert.Equal(t, term.ColorCyan, cell.Bg)
					assert.Equal(t, term.ColorBlue, cell.Fg)
				}
			}
		}
	})
}
