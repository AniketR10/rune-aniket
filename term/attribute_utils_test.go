package term

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAttributesUnion(t *testing.T) {
	tsuite := []struct {
		inA, inB Attributes
		wantOut  Attributes
	}{
		{},

		{Attributes{Fg: ColorRed}, Attributes{Fg: ColorDefault}, Attributes{Fg: ColorRed}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: ColorDefault}, Attributes{Bg: ColorRed}},

		{Attributes{Fg: ColorWhite}, Attributes{Fg: ColorRed}, Attributes{Fg: ColorRed}},
		{Attributes{Bg: ColorWhite}, Attributes{Bg: ColorRed}, Attributes{Bg: ColorRed}},

		{Attributes{Fg: ColorRed | AttrBold | AttrUnderline | AttrReverse}, Attributes{Fg: ColorRed | AttrBold | AttrUnderline | AttrReverse},
			Attributes{Fg: ColorRed | AttrBold | AttrUnderline | AttrReverse}},
		{Attributes{Bg: ColorRed | AttrBold | AttrUnderline | AttrReverse}, Attributes{Bg: ColorRed | AttrBold | AttrUnderline | AttrReverse},
			Attributes{Bg: ColorRed | AttrBold | AttrUnderline | AttrReverse}},

		{Attributes{Fg: AttrBold}, Attributes{Fg: ColorRed}, Attributes{Fg: ColorRed | AttrBold}},
		{Attributes{Bg: AttrBold}, Attributes{Bg: ColorRed}, Attributes{Bg: ColorRed | AttrBold}},
		{Attributes{Fg: AttrUnderline}, Attributes{Fg: ColorRed}, Attributes{Fg: ColorRed | AttrUnderline}},
		{Attributes{Bg: AttrUnderline}, Attributes{Bg: ColorRed}, Attributes{Bg: ColorRed | AttrUnderline}},
		{Attributes{Fg: AttrReverse}, Attributes{Fg: ColorRed}, Attributes{Fg: ColorRed | AttrReverse}},
		{Attributes{Bg: AttrReverse}, Attributes{Bg: ColorRed}, Attributes{Bg: ColorRed | AttrReverse}},

		{Attributes{Fg: ColorRed}, Attributes{Fg: AttrBold}, Attributes{Fg: ColorRed | AttrBold}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: AttrBold}, Attributes{Bg: ColorRed | AttrBold}},
		{Attributes{Fg: ColorRed}, Attributes{Fg: AttrUnderline}, Attributes{Fg: ColorRed | AttrUnderline}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: AttrUnderline}, Attributes{Bg: ColorRed | AttrUnderline}},
		{Attributes{Fg: ColorRed}, Attributes{Fg: AttrReverse}, Attributes{Fg: ColorRed | AttrReverse}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: AttrReverse}, Attributes{Bg: ColorRed | AttrReverse}},

		{Attributes{Bg: AttrReverse | AttrBold | AttrUnderline}, Attributes{Bg: ColorRed},
			Attributes{Bg: ColorRed | AttrReverse | AttrBold | AttrUnderline}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: AttrReverse | AttrBold | AttrUnderline},
			Attributes{Bg: ColorRed | AttrReverse | AttrBold | AttrUnderline}},

		{Attributes{Fg: ColorDefault}, Attributes{Fg: ColorRed}, Attributes{Fg: ColorRed}},
		{Attributes{Bg: ColorDefault}, Attributes{Bg: ColorRed}, Attributes{Bg: ColorRed}},
	}

	for i, tcase := range tsuite {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actualOut := AttributesUnion(tcase.inA, tcase.inB)
			assert.Equal(t, tcase.wantOut, actualOut)
		})
	}
}

func TestAttributesDifference(t *testing.T) {
	tsuite := []struct {
		inA, inB Attributes
		wantOut  Attributes
	}{
		{},

		{Attributes{Fg: ColorRed | AttrBold | AttrUnderline | AttrReverse}, Attributes{Fg: ColorRed | AttrBold | AttrUnderline | AttrReverse},
			Attributes{Fg: ColorDefault}},
		{Attributes{Bg: ColorRed | AttrBold | AttrUnderline | AttrReverse}, Attributes{Bg: ColorRed | AttrBold | AttrUnderline | AttrReverse},
			Attributes{Bg: ColorDefault}},

		{Attributes{Fg: ColorRed}, Attributes{Fg: ColorDefault}, Attributes{Fg: ColorRed}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: ColorDefault}, Attributes{Bg: ColorRed}},

		{Attributes{Fg: ColorWhite}, Attributes{Fg: ColorRed}, Attributes{Fg: ColorWhite}},
		{Attributes{Bg: ColorWhite}, Attributes{Bg: ColorRed}, Attributes{Bg: ColorWhite}},

		{Attributes{Fg: ColorWhite}, Attributes{Fg: ColorWhite}, Attributes{Fg: ColorDefault}},
		{Attributes{Bg: ColorWhite}, Attributes{Bg: ColorWhite}, Attributes{Bg: ColorDefault}},

		{Attributes{Fg: AttrBold}, Attributes{Fg: ColorRed}, Attributes{Fg: AttrBold}},
		{Attributes{Bg: AttrBold}, Attributes{Bg: ColorRed}, Attributes{Bg: AttrBold}},
		{Attributes{Fg: AttrUnderline}, Attributes{Fg: ColorRed}, Attributes{Fg: AttrUnderline}},
		{Attributes{Bg: AttrUnderline}, Attributes{Bg: ColorRed}, Attributes{Bg: AttrUnderline}},
		{Attributes{Fg: AttrReverse}, Attributes{Fg: ColorRed}, Attributes{Fg: AttrReverse}},
		{Attributes{Bg: AttrReverse}, Attributes{Bg: ColorRed}, Attributes{Bg: AttrReverse}},

		{Attributes{Fg: ColorRed}, Attributes{Fg: AttrBold}, Attributes{Fg: ColorRed}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: AttrBold}, Attributes{Bg: ColorRed}},
		{Attributes{Fg: ColorRed}, Attributes{Fg: AttrUnderline}, Attributes{Fg: ColorRed}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: AttrUnderline}, Attributes{Bg: ColorRed}},
		{Attributes{Fg: ColorRed}, Attributes{Fg: AttrReverse}, Attributes{Fg: ColorRed}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: AttrReverse}, Attributes{Bg: ColorRed}},

		{Attributes{Bg: AttrReverse | AttrBold | AttrUnderline}, Attributes{Bg: ColorRed},
			Attributes{Bg: AttrReverse | AttrBold | AttrUnderline}},
		{Attributes{Bg: ColorRed}, Attributes{Bg: AttrReverse | AttrBold | AttrUnderline},
			Attributes{Bg: ColorRed}},

		{Attributes{Fg: ColorDefault}, Attributes{Fg: ColorRed}, Attributes{Fg: ColorDefault}},
		{Attributes{Bg: ColorDefault}, Attributes{Bg: ColorRed}, Attributes{Bg: ColorDefault}},
	}

	for i, tcase := range tsuite {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actualOut := AttributesDifference(tcase.inA, tcase.inB)
			assert.Equal(t, tcase.wantOut, actualOut)
		})
	}
}

func BenchmarkAttributesOperations(b *testing.B) {
	attr := Attributes{Fg: ColorDefault, Bg: ColorDefault}
	for i := 0; i < b.N; i++ {
		attr = AttributesUnion(attr, Attributes{Fg: ColorRed | AttrBold, Bg: AttrUnderline})
		attr = AttributesDifference(attr, Attributes{Fg: AttrBold, Bg: AttrReverse})
		attr = AttributesUnion(attr, Attributes{Fg: ColorGreen | AttrUnderline, Bg: ColorBlack})
	}
}

// this is how we used to do it before the previous helpers were added.
// The results are incorrect when colors are set though, but this bench
// is to be able to compare the two methods.
func BenchmarkAttributesLegacy(b *testing.B) {
	attr := Attributes{Fg: ColorDefault, Bg: ColorDefault}
	for i := 0; i < b.N; i++ {
		attr.Fg |= ColorRed | AttrBold
		attr.Bg |= AttrUnderline
		attr.Fg &^= AttrBold
		attr.Bg &^= AttrReverse
		attr.Fg |= ColorGreen | AttrUnderline
		attr.Bg |= ColorBlack
	}
}
