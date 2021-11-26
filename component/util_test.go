package component

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testString(t *testing.T,
	fn func(string) tui.Component,
	width, height int, in, out string,
) {
	w := term.NewStringWriter(width, height)
	comp := fn(in)
	comp.Resize(width, height)
	comp.Draw(w)
	require.NoError(t, w.Flush())
	assert.Equal(t, out, w.String())
}

func TestStringCentered(t *testing.T) {
	tcases := []struct {
		in  string
		out string
	}{
		{
			in:  "aaaa",
			out: "     \n     \naaaa \n     \n     ",
		},
		{
			in:  "XXXXXXXXXXXXXXXX\nXXXXXXXXXX\nXXXXXXXXXX",
			out: "     \nXXXXX\nXXXXX\nXXXXX\n     ",
		},
		{
			in:  "X\nX\nX\nX\nX\nX\nX\nX\n",
			out: "  X  \n  X  \n  X  \n  X  \n  X  ",
		},
		{
			in:  "X\nX\nX\nX\nX\nX\nX\nX\n",
			out: "  X  \n  X  \n  X  \n  X  \n  X  ",
		},
		{
			in:  "XXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\n",
			out: "XXXXX\nXXXXX\nXXXXX\nXXXXX\nXXXXX",
		},
		{
			in:  "a",
			out: "     \n     \n  a  \n     \n     ",
		},
	}

	for _, tcase := range tcases {
		testString(t, StringCentered, 5, 5, tcase.in, tcase.out)
	}
}

func TestString(t *testing.T) {
	tcases := []struct {
		in  string
		out string
	}{
		{
			in:  "aaaa",
			out: "aaaa \n     \n     \n     \n     ",
		},
		{
			in:  "XXXXXXXXXXXXXXXX\nXXXXXXXXXX\nXXXXXXXXXX",
			out: "XXXXX\n     \n     \n     \n     ",
		},
		{
			in:  "X\nX\nX\nX\nX\nX\nX\nX\n",
			out: "X X X\n     \n     \n     \n     ",
		},
		{
			in:  "XXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\nXXXXXXXXXXX\n",
			out: "XXXXX\n     \n     \n     \n     ",
		},
		{
			in:  "a",
			out: "a    \n     \n     \n     \n     ",
		},
	}

	for _, tcase := range tcases {
		testString(t, String, 5, 5, tcase.in, tcase.out)
	}
}

func benchmarkString(b *testing.B, fortunes int) {
	var builder strings.Builder
	for i := 0; i < fortunes; i++ {
		_, _ = builder.WriteString(fortune)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = String(builder.String())
	}
}

func BenchmarkString1(b *testing.B) {
	benchmarkString(b, 1)
}
func BenchmarkString10(b *testing.B) {
	benchmarkString(b, 10)
}
func BenchmarkString100(b *testing.B) {
	benchmarkString(b, 100)
}
func BenchmarkString1000(b *testing.B) {
	benchmarkString(b, 1000)
}
func BenchmarkString10000(b *testing.B) {
	benchmarkString(b, 10000)
}
