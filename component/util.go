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
// and uses c as the background rune.
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

// String converts a string into a static tui.Compontent.
func String(str string) tui.Component {
	return StringBackground(str, 0)
}

// StaticString converts a string into a static, efficient tui.Component
// which draws the given string. If the string needs to be centered dynamically,
// use String instead. This method is ~20% faster than String for
// small payloads and it does ~20% less allocations.
func StaticString(str string) tui.Component {
	cells := cell.StringToCells(str)
	return &stringComp{cells: cells}
}

// StaticStringAttr converts a string into a static, efficient tui.Component
// which draws str along with attr. See StaticString for more information.
func StaticStringAttr(str string, attr term.Attributes) tui.Component {
	cells := cell.StringToCells(str)
	return &stringComp{cells: cells, attr: attr}
}
