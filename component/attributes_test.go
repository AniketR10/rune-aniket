package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
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
		s.SetAttrAt(term.Coordinates{X: 2, Y: 2}, term.Attributes{Fg: tcell.ColorRed})

		s.Resize(2, 2)

		// test that it doesn't panic
		s.Draw(w)
	})

	t.Run("happy path", func(t *testing.T) {
		w := cell.NewBufferWriter(context.Background(), 4, 4)
		s := WithAttrSetter(&TestComponent{Ch: 'a'})
		s.SetAttr(term.Attributes{Fg: tcell.ColorBlue, Bg: tcell.ColorNavy})
		s.SetAttrAt(term.Coordinates{X: 3, Y: 3},
			term.Attributes{
				Fg:    tcell.ColorRed,
				Bg:    tcell.ColorGreen,
				Attrs: tcell.AttrBold | tcell.AttrUnderline,
			})

		s.Resize(4, 4)
		s.Draw(w)

		for y, row := range w.RawCells() {
			for x, cell := range row {
				if y == 3 && x == 3 {
					assert.Equal(t, tcell.ColorGreen, cell.Bg)
					assert.Equal(t, tcell.ColorRed, cell.Fg)
					assert.True(t, cell.Attrs&tcell.AttrUnderline != 0)
					assert.True(t, cell.Attrs&tcell.AttrBold != 0)
				} else {
					assert.Equal(t, tcell.ColorNavy, cell.Bg)
					assert.Equal(t, tcell.ColorBlue, cell.Fg)
				}
			}
		}
	})
}
