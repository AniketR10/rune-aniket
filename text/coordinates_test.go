package text

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

func TestScrollToWindowCoordinates(t *testing.T) {
	suite := []struct {
		description string
		inScroll    func(t *testing.T) *component.Scroll
		scroll      term.Coordinates
		window      term.Coordinates
	}{
		{"no wrap, no offset, within view bounds, start of file",
			makeScroll(false, 100, 100, 0, 0), term.Coordinates{}, term.Coordinates{}},
		{"no wrap, no offset, within view bounds, end of file",
			makeScroll(false, 100, 100, 0, 0), term.Coordinates{Y: 4, X: 13}, term.Coordinates{Y: 4, X: 13}},
		{"no wrap, no offset, within view bounds, past end of file",
			makeScroll(false, 100, 100, 0, 0), term.Coordinates{Y: 5, X: 1}, term.Coordinates{Y: 5, X: 1}},
		{"no wrap, no offset, within view bounds, past end of one line",
			makeScroll(false, 100, 100, 0, 0), term.Coordinates{Y: 0, X: 100}, term.Coordinates{Y: 0, X: 100}},
		{"no wrap, no offset, outside view bounds, end of file",
			makeScroll(false, 10, 3, 0, 0), term.Coordinates{Y: 4, X: 13}, term.Coordinates{Y: 4, X: 13}},
		{"no wrap, no offset, outside view bounds, past end of file",
			makeScroll(false, 10, 3, 0, 0), term.Coordinates{Y: 5, X: 1}, term.Coordinates{Y: 5, X: 1}},
		{"no wrap, no offset, outside view bounds, past end of one line",
			makeScroll(false, 10, 3, 0, 0), term.Coordinates{Y: 0, X: 100}, term.Coordinates{Y: 0, X: 100}},
		{"no wrap, with offset, outside view bounds, end of file",
			makeScroll(false, 10, 3, 1, 1), term.Coordinates{Y: 4, X: 13}, term.Coordinates{Y: 3, X: 12}},
		{"no wrap, with offset, outside view bounds, past end of file",
			makeScroll(false, 10, 3, 1, 1), term.Coordinates{Y: 5, X: 1}, term.Coordinates{Y: 4, X: 0}},
		{"no wrap, with offset, outside view bounds, past end of one line",
			makeScroll(false, 10, 3, 1, 1), term.Coordinates{Y: 0, X: 100}, term.Coordinates{Y: -1, X: 99}},

		{"wrap, no offset, within view bounds, start of file",
			makeScroll(true, 100, 100, 0, 0), term.Coordinates{}, term.Coordinates{}},
		{"wrap, no offset, within view bounds, end of file",
			makeScroll(true, 100, 100, 0, 0), term.Coordinates{Y: 4, X: 13}, term.Coordinates{Y: 4, X: 13}},
		{"wrap, no offset, within view bounds, past end of file",
			makeScroll(true, 100, 100, 0, 0), term.Coordinates{Y: 5, X: 1}, term.Coordinates{Y: 5, X: 1}},
		{"wrap, no offset, within view bounds, past end of one line",
			makeScroll(true, 100, 100, 0, 0), term.Coordinates{Y: 0, X: 100}, term.Coordinates{Y: 1, X: 0}},

		{"wrap, no offset, outside view bounds, end of file",
			makeScroll(true, 10, 3, 0, 0), term.Coordinates{Y: 4, X: 13}, term.Coordinates{Y: 7, X: 3}},
		{"wrap, no offset, outside view bounds, past end of file",
			makeScroll(true, 10, 3, 0, 0), term.Coordinates{Y: 5, X: 1}, term.Coordinates{Y: 8, X: 1}},
		{"wrap, no offset, outside view bounds, past end of one line",
			makeScroll(true, 10, 3, 0, 0), term.Coordinates{Y: 0, X: 100}, term.Coordinates{Y: 10, X: 0}},

		{"wrap, with offset, outside view bounds, end of file",
			makeScroll(true, 10, 3, 1, 1), term.Coordinates{Y: 4, X: 13}, term.Coordinates{Y: 6, X: 2}},
		{"wrap, with offset, outside view bounds, past end of file",
			makeScroll(true, 10, 3, 1, 1), term.Coordinates{Y: 5, X: 1}, term.Coordinates{Y: 7, X: 0}},
		{"wrap, with offset, outside view bounds, past end of one line",
			makeScroll(true, 10, 3, 1, 1), term.Coordinates{Y: 0, X: 100}, term.Coordinates{Y: 9, X: -1}},

		{"wrap, no offset, outside view bounds, line in the middle, at the end of line",
			makeScroll(true, 10, 3, 0, 0), term.Coordinates{Y: 2, X: 13}, term.Coordinates{Y: 4, X: 3}},
		{"wrap, no offset, outside view bounds, line in the middle, past the end of file, would wrap, past end of one line",
			makeScroll(true, 10, 3, 0, 0), term.Coordinates{Y: 5, X: 13}, term.Coordinates{Y: 9, X: 3}},
		{"no wrap, end of file offset, first line",
			makeScroll(false, 10, 3, 2, 0), term.Coordinates{Y: 0, X: 0}, term.Coordinates{Y: -2, X: 0}},
		{"wrap, end of file offset, first line",
			makeScroll(true, 10, 3, 5, 0), term.Coordinates{Y: 0, X: 0}, term.Coordinates{Y: -5, X: 0}},
		{"wrap, past end of file offset, first line",
			makeScroll(true, 10, 3, 6, 1), term.Coordinates{Y: 0, X: 0}, term.Coordinates{Y: -6, X: -1}},
		{"wrap, halfway through line offset, first line",
			makeScroll(true, 10, 3, 4, 0), term.Coordinates{Y: 0, X: 0}, term.Coordinates{Y: -4, X: 0}},
		{"wrap, (2nd) halfway through line offset, first line",
			makeScroll(true, 10, 3, 3, 0), term.Coordinates{Y: 0, X: 0}, term.Coordinates{Y: -3, X: 0}},
		{"3rd wrap, halfway through line offset, negative window pos",
			makeScroll(true, 10, 3, 2, 0), term.Coordinates{Y: 0, X: 10}, term.Coordinates{Y: -1, X: 0}},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			require.Equal(t, test.window,
				ScrollToWindowCoordinates(test.inScroll(t), test.scroll), "scroll to window")
			// resulting position is ambiguous, this should not happen
			// in a real case scaneario anyway
			if strings.Contains(test.description, "past end of one line") ||
				strings.Contains(test.description, "past end of file") {
				return
			}
			assert.Equal(t, test.scroll,
				WindowToScrollCoordinates(test.inScroll(t), test.window), "window to scroll")
		})
	}
}

// test the cases that weren't tested above
func TestWindowCoordinatesToScrollCoordinatesWrapLastLine(t *testing.T) {
	suite := []struct {
		yoffset int
		xoffset int
		wpos    term.Coordinates
		spos    term.Coordinates
	}{
		{0, 0, term.Coordinates{Y: 5, X: 9}, term.Coordinates{Y: 3, X: 9}},
		{0, 0, term.Coordinates{Y: 6, X: 0}, term.Coordinates{Y: 4, X: 0}},
		{0, 0, term.Coordinates{Y: 7, X: 0}, term.Coordinates{Y: 4, X: 10}},
		{0, 0, term.Coordinates{Y: 8, X: 0}, term.Coordinates{Y: 4, X: 14}},
		{0, 0, term.Coordinates{Y: 9, X: 0}, term.Coordinates{Y: 4, X: 24}},
		{0, 0, term.Coordinates{Y: 10, X: 0}, term.Coordinates{Y: 4, X: 34}},

		{1, 1, term.Coordinates{Y: 4, X: 8}, term.Coordinates{Y: 3, X: 9}},
		{1, 1, term.Coordinates{Y: 5, X: -1}, term.Coordinates{Y: 4, X: 0}},
		{1, 1, term.Coordinates{Y: 6, X: -1}, term.Coordinates{Y: 4, X: 10}},
		{1, 1, term.Coordinates{Y: 7, X: -1}, term.Coordinates{Y: 4, X: 14}},
		{1, 1, term.Coordinates{Y: 8, X: -1}, term.Coordinates{Y: 4, X: 24}},
		{1, 1, term.Coordinates{Y: 9, X: -1}, term.Coordinates{Y: 4, X: 34}},
	}

	for _, test := range suite {
		scroll := makeScroll(true, 10, 3, test.yoffset, test.xoffset)(t)
		actual := WindowToScrollCoordinates(scroll, test.wpos)
		assert.Equal(t, test.spos, actual)
	}
}

func TestWindowCoordinatesPanicDeleteRow(t *testing.T) {
	scroll := makeScroll(true, 10, 3, 0, 0)(t)
	scroll.Buffer().DeleteRow(0)
	assert.NotPanics(t, func() {
		WindowToScrollCoordinates(scroll, term.Coordinates{Y: 8, X: 0})
		// do not assert result as it will always be incorrect
	})
}

func makeScroll(wrap bool, width, height, offsetY, offsetX int) func(t *testing.T) *component.Scroll {
	const content = `AAAAAAAAAAAAA
BBBBBBB
CCCCCCCCCCCCCC
DDDDDDD
EEEEEEEEEEEEEE`
	return makeScrollContent(wrap, width, height, offsetY, offsetX, content)
}

func makeScrollContent(wrap bool, width, height, offsetY, offsetX int, content string) func(t *testing.T) *component.Scroll {
	return func(t *testing.T) *component.Scroll {
		buf := cell.NewBuffer()
		buf.WriteString(content)
		ret := component.NewScroll(buf)
		ret.Wrap = wrap
		ret.Resize(width, height)
		// necessary for some wrap to work for SeekTo and scroll.Wraps usage
		ret.Draw(term.NewStringWriter(width, height))
		if offsetY != 0 {
			require.True(t, ret.SeekVertical(offsetY))
		}
		if offsetX != 0 {
			require.True(t, ret.SeekHorizontal(offsetX))
		}
		return ret
	}
}
