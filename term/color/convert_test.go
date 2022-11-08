package color

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/term"
)

func TestRGBToAttribute(t *testing.T) {
	tsuite := []struct {
		r, g, b  uint8
		expected term.Attribute
	}{
		{255, 255, 255, term.ColorWhite},
		{0, 0, 0, term.ColorBlack},
		{255, 0, 0, term.ColorRed},
		{0, 255, 0, term.ColorGreen},
		{255, 255, 0, term.ColorYellow},
		{0, 0, 255, term.ColorBlue},
		{255, 0, 255, term.ColorMagenta},
		{0, 255, 255, term.ColorCyan},
		{254, 255, 255, 232},
		{1, 0, 0, 17},
		{254, 0, 0, 197},
		{1, 255, 0, 47},
		{254, 255, 0, 227},
		{1, 255, 255, 52},
	}

	for _, tcase := range tsuite {
		desc := fmt.Sprintf("RGB(%d,%d,%d)=>term.Attribute(%d)", tcase.r, tcase.g, tcase.b, tcase.expected)
		t.Run(desc, func(t *testing.T) {
			actual := RGBToAttribute(tcase.r, tcase.g, tcase.b)
			assert.Equal(t, int(tcase.expected), int(actual))
		})
	}
}
