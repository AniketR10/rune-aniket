package component

import (
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/stretchr/testify/assert"
)

func TestResponsiveStringDraw(t *testing.T) {
	t.Run("should behave like String with single line strings and enough space", func(t *testing.T) {
		tcases := []struct {
			in  string
			out string
			cfg StringConfig
		}{
			{
				in:  "aaaa",
				out: "aaaa \n     \n     \n     \n     ",
			},
			{
				in:  "X\nX\nX\nX\nX\nX\nX\nX\n",
				out: "X    \nX    \nX    \nX    \nX    ",
			},
			{
				in:  "a",
				out: "a    \n     \n     \n     \n     ",
			},
		}

		for _, tcase := range tcases {
			testString(t, func(in string) tui.Component {
				return StringResponsive(in, tcase.cfg)
			}, 5, 5, tcase.in, tcase.out)
		}
	})

	t.Run("should wrap lines around", func(t *testing.T) {
		tcases := []struct {
			in  string
			out string
			cfg StringConfig
		}{
			{
				in:  "XXXXXXXXXX\nBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDD\nEEEEEEEEEEE\nFFFFFFFFFF\n",
				out: "XXXXX\nXXXXX\nBBBBB\nBBBBB\nCCCCC",
			},
			{
				in:  "XXXXXXXXXXXXXXXX\nYYYYYYYYYnZZZZZZZZZ",
				out: "XXXXX\nXXXXX\nXXXXX\nX    \nYYYYY",
			},
			{
				in:  "X\n1111 \n222222222222222222222222222222222",
				out: "X    \n1111 \n22222\n22222\n22222",
			},
		}

		for _, tcase := range tcases {
			testString(t, func(in string) tui.Component {
				return StringResponsive(in, tcase.cfg)
			}, 5, 5, tcase.in, tcase.out)
		}
	})
}

func TestResponsiveStringHeight(t *testing.T) {
	tcases := []struct {
		in    string
		width int
		out   int
	}{
		{
			in:    "XXXXXXXXXX\nBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDD\nEEEEEEEEEEE\nFFFFFFFFFF\n",
			width: 5,
			out:   15,
		},
		{
			in:    "X",
			width: 5,
			out:   1,
		},
		{
			in:    "X\nX\nX\nX\n",
			width: 10,
			out:   5,
		},
		{
			in:    "XXXXXXXXXX",
			width: 10,
			out:   1,
		},
	}

	for _, tcase := range tcases {
		s := StringResponsive(tcase.in, StringConfig{})
		out := s.Height(tcase.width)
		assert.Equal(t, tcase.out, out)
	}
}
