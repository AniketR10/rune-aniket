package termutil

import (
	"fmt"
	"strconv"

	"github.com/ernestrc/tcell/v3"
	"unstable.build/go-tui/term"
)

type Theme struct {
	Default term.Attributes
}

var (
	map4Bit = map[uint8]tcell.Color{
		30:  tcell.ColorBlack,
		31:  tcell.ColorRed,
		32:  tcell.ColorGreen,
		33:  tcell.ColorYellow,
		34:  tcell.ColorBlue,
		35:  tcell.ColorPurple,
		36:  tcell.ColorNavy,
		37:  tcell.ColorWhite,
		90:  tcell.ColorBlack,
		91:  tcell.ColorRed,
		92:  tcell.ColorGreen,
		93:  tcell.ColorYellow,
		94:  tcell.ColorBlue,
		95:  tcell.ColorPurple,
		96:  tcell.ColorNavy,
		97:  tcell.ColorWhite,
		40:  tcell.ColorBlack,
		41:  tcell.ColorRed,
		42:  tcell.ColorGreen,
		43:  tcell.ColorYellow,
		44:  tcell.ColorBlue,
		45:  tcell.ColorPurple,
		46:  tcell.ColorNavy,
		47:  tcell.ColorWhite,
		100: tcell.ColorBlack,
		101: tcell.ColorRed,
		102: tcell.ColorGreen,
		103: tcell.ColorYellow,
		104: tcell.ColorBlue,
		105: tcell.ColorPurple,
		106: tcell.ColorNavy,
		107: tcell.ColorWhite,
	}
)

func (t *Theme) ColourFrom4Bit(code uint8) tcell.Color {
	colour, ok := map4Bit[code]
	if !ok {
		return tcell.ColorDefault
	}
	return colour
}

func (t *Theme) ColourFrom8Bit(n string) (tcell.Color, error) {
	index, err := strconv.Atoi(n)
	if err != nil {
		return 0, err
	}

	return tcell.PaletteColor(index), nil
}

func (t *Theme) ColourFrom24Bit(r, g, b string) (tcell.Color, error) {
	ri, err := strconv.Atoi(r)
	if err != nil {
		return 0, err
	}
	gi, err := strconv.Atoi(g)
	if err != nil {
		return 0, err
	}
	bi, err := strconv.Atoi(b)
	if err != nil {
		return 0, err
	}

	return tcell.NewColor(int32(ri), int32(gi), int32(bi)), nil
}

func (t *Theme) ColourFromAnsi(ansi []string, bg bool) (tcell.Color, error) {
	if len(ansi) == 0 {
		return 0, fmt.Errorf("invalid ansi colour code")
	}

	switch ansi[0] {
	case "2":
		if len(ansi) != 4 {
			return 0, fmt.Errorf("invalid 24-bit ansi colour code")
		}
		return t.ColourFrom24Bit(ansi[1], ansi[2], ansi[3])
	case "5":
		if len(ansi) != 2 {
			return 0, fmt.Errorf("invalid 8-bit ansi colour code")
		}
		return t.ColourFrom8Bit(ansi[1])
	default:
		return 0, fmt.Errorf("invalid ansi colour code")
	}
}
