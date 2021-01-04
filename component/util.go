package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

type stringComp struct {
	width, height int
	cells         [][]term.Cell
	attr          term.Attributes
}

func (s *stringComp) Resize(width, height int) {
	s.width, s.height = width, height
}

func (s *stringComp) Draw(w term.Writer) {
	for y, r := range s.cells {
		if y >= s.height {
			break
		}
		for x, c := range r {
			if x >= s.width {
				break
			}
			if c.Ch == 0 {
				continue
			}
			w.SetCell(term.Coordinates{X: x, Y: y},
				term.Cell{Bg: s.attr.Bg, Fg: s.attr.Fg, Ch: c.Ch})
		}
	}
}

// StringBackground converts a string into a static tui.Compontent,
// and uses c as the background rune. It processes newlines and so draws
// the string multi line if applicable. It also centers the string vertically
// and horizontally.
func StringBackground(str string, c rune) tui.Component {
	cells := cell.StringToCells(str)
	comp := stringComp{cells: cells}
	background := term.Cell{Ch: c}

	var width int
	for _, row := range cells {
		if len(row) > width {
			width = len(row)
		}
	}

	return NewSpan(WithBackground(&comp, background), SpanConfig{
		PadVertical:   -len(cells),
		PadHorizontal: -width,
	})
}

// StringCentered converts a string into a static tui.Compontent.
// It processes newlines and so draws the string multi line if applicable.
// It also centers the string vertically and horizontally.
func StringCentered(str string) tui.Component {
	return StringBackground(str, 0)
}

// String converts a string into a very efficient top left centered one line tui.Component
// which draws the given string. If the string needs to be centered dynamically,
// or drawn multi-line use StringCentered instead.
func String(str string) tui.Component {
	row := make([]term.Cell, len(str))
	for i, r := range str {
		row[i] = term.Cell{Ch: r}
	}
	return &stringComp{cells: [][]term.Cell{row}}
}

// StringAttr converts a string into a very efficient top left centered multi-line tui.Component
// which draws str along with attr. See String for more information.
func StringAttr(str string, attr term.Attributes) tui.Component {
	row := make([]term.Cell, len(str))
	for i, r := range str {
		row[i] = term.Cell{Ch: r}
	}
	return &stringComp{cells: [][]term.Cell{row}, attr: attr}
}
