package term

import (
	"github.com/unstablebuild/tcell/v3"
)

// AttributesDifference computes the set difference between a and b,
// that is it returns a set of attributes that contain
// all the bit flags set in a but not set in b, and returns
// ColorDefault if a's color is equal to b's color or returns
// the color set in a.
func AttributesDifference(a, b Attributes) Attributes {
	retFgColor := a.Fg
	retBgColor := a.Bg
	if a.Fg == b.Fg {
		retFgColor = tcell.ColorDefault
	}
	if a.Bg == b.Bg {
		retBgColor = tcell.ColorDefault
	}
	return Attributes{Fg: retFgColor, Bg: retBgColor, Attrs: (a.Attrs &^ b.Attrs)}
}

// AttributesUnion computes the set union between a and b,
// that is it returns a set of attributes that contain
// all the bit flags set in a, b or both, and uses the color
// defined in b or if not set, uses the color in a.
func AttributesUnion(a, b Attributes) Attributes {
	retFgColor := b.Fg
	retBgColor := b.Bg
	if b.Fg == tcell.ColorDefault {
		retFgColor = a.Fg
	}
	if b.Bg == tcell.ColorDefault {
		retBgColor = a.Bg
	}
	return Attributes{Fg: retFgColor, Bg: retBgColor, Attrs: (a.Attrs | b.Attrs)}
}
