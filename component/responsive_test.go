package component

import (
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			t.Run("StringResponsive", func(t *testing.T) {
				testString(t, func(in string) tui.Component {
					return StringResponsive(in, tcase.cfg)
				}, 5, 5, tcase.in, tcase.out)
			})

			t.Run("BufferResponsive", func(t *testing.T) {
				testString(t, func(in string) tui.Component {
					b := cell.NewBuffer()
					b.WriteString(in)
					return BufferResponsive(b, tcase.cfg)
				}, 5, 5, tcase.in, tcase.out)
			})
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
			t.Run("StringResponsive", func(t *testing.T) {
				testString(t, func(in string) tui.Component {
					return StringResponsive(in, tcase.cfg)
				}, 5, 5, tcase.in, tcase.out)
			})

			t.Run("BufferResponsive", func(t *testing.T) {
				testString(t, func(in string) tui.Component {
					b := cell.NewBuffer()
					b.WriteString(in)
					return BufferResponsive(b, tcase.cfg)
				}, 5, 5, tcase.in, tcase.out)
			})
		}
	})
}

func TestResponsiveHeight(t *testing.T) {
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
		{
			in:    "XXXXXXXXXX",
			width: -1,
			out:   0,
		},
		{
			in:    "XXXXXXXXXX",
			width: 0, // could trigger division by zero
			out:   0,
		},
	}

	for _, tcase := range tcases {
		t.Run("StringResponsive", func(t *testing.T) {
			s := StringResponsive(tcase.in, StringConfig{})
			out := s.Height(tcase.width)
			assert.Equal(t, tcase.out, out)
		})
		t.Run("BufferResponsive", func(t *testing.T) {
			b := cell.CellsToBuffer(nil)
			b.WriteString(tcase.in)
			s := BufferResponsive(b, StringConfig{})
			out := s.Height(tcase.width)
			assert.Equal(t, tcase.out, out)
		})
	}
}

func TestBufferWithUpdates(t *testing.T) {
	t.Run("Height", func(t *testing.T) {
		b := cell.CellsToBuffer(nil)
		b.WriteString("aa")
		s := BufferResponsive(b, StringConfig{})

		height := s.Height(1)
		assert.Equal(t, 2, height)

		b.WriteString("a")

		height = s.Height(1)
		assert.Equal(t, 3, height)
	})

	t.Run("Draw", func(t *testing.T) {
		w := term.NewStringWriter(5, 5)
		b := cell.CellsToBuffer(nil)
		b.WriteString("a\nb\nc")
		s := BufferResponsive(b, StringConfig{})
		s.Resize(4, 4)

		s.Draw(w)
		require.NoError(t, w.Flush())
		assert.Equal(t, "a    \nb    \nc    \n     \n     ", w.String())

		b.WriteString("xyz")

		w = term.NewStringWriter(5, 5)
		s.Draw(w)
		require.NoError(t, w.Flush())
		assert.Equal(t, "a    \nb    \ncxyz \n     \n     ", w.String())
	})
}
