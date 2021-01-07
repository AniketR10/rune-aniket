package component

import (
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
)

func TestDrawAttributes(t *testing.T) {
	t.Run("draws cells correctly and respecting bounds", func(t *testing.T) {
		w := term.NewStringWriter(9, 5)
		u := &TestComponent{Ch: 'X'}
		s := WithAttrSetter(u)

		s.Resize(8, 4)

		tests := []testCase{
			{
				nil, `
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
         `,
			},
		}

		testWorkflow(t, s, w, tests)
	})
}

func TestAttrSetter(t *testing.T) {
	w := cell.NewBufferWriter(4, 4)
	s := WithAttrSetter(&TestComponent{Ch: 'a'})
	s.SetAttr(term.Attributes{Fg: term.ColorBlue, Bg: term.ColorCyan})
	s.SetAttrAt(term.Coordinates{X: 1, Y: 1}, term.Attributes{Fg: term.ColorRed | term.AttrBold, Bg: term.ColorGreen | term.AttrUnderline})

	s.Resize(4, 4)
	s.Draw(w)

	for y, row := range w.RawCells() {
		for x, cell := range row {
			if y == 1 && x == 1 {
				assert.True(t, (term.ColorGreen|term.AttrUnderline)&cell.Bg != 0)
				assert.True(t, (term.ColorRed|term.AttrBold)&cell.Fg != 0)
			} else {
				assert.Equal(t, term.ColorCyan, cell.Bg)
				assert.Equal(t, term.ColorBlue, cell.Fg)
			}
		}
	}
}
