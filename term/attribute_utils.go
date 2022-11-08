package term

// AttributesDifference computes the set difference between a and b,
// that is it returns a set of attributes that contain
// all the bit flags set in a but not set in b, and returns
// ColorDefault if a's color is equal to b's color or returns
// the color set in a.
func AttributesDifference(a, b Attributes) Attributes {
	return Attributes{
		Fg: AttributeDifference(a.Fg, b.Fg),
		Bg: AttributeDifference(a.Bg, b.Bg),
	}
}

// AttributeDifference computes the set difference between a and b,
// that is it returns a set of attributes that contain
// all the bit flags set in a but not set in b, and returns
// ColorDefault if a's color is equal to b's color or returns
// the color set in a.
func AttributeDifference(a, b Attribute) Attribute {
	aAttrFlags := a&AttrBold | a&AttrUnderline | a&AttrReverse
	aColor := a &^ (AttrBold | AttrReverse | AttrUnderline)

	bAttrFlags := b&AttrBold | b&AttrUnderline | b&AttrReverse
	bColor := b &^ (AttrBold | AttrReverse | AttrUnderline)

	retColor := aColor
	if aColor == bColor {
		retColor = ColorDefault
	}

	return retColor | (aAttrFlags &^ bAttrFlags)
}

// AttributesUnion computes the set union between a and b,
// that is it returns a set of attributes that contain
// all the bit flags set in a, b or both, and uses the color
// defined in b or if not set, uses the color in a.
func AttributesUnion(a, b Attributes) Attributes {
	return Attributes{
		Fg: AttributeUnion(a.Fg, b.Fg),
		Bg: AttributeUnion(a.Bg, b.Bg),
	}
}

// AttributeUnion computes the set union between a and b,
// that is it returns a set of attributes that contain
// all the bit flags set in a, b or both, and uses the color
// defined in b or if not set, uses the color in a.
func AttributeUnion(a, b Attribute) Attribute {
	aAttrFlags := a&AttrBold | a&AttrUnderline | a&AttrReverse
	aColor := a &^ (AttrBold | AttrReverse | AttrUnderline)

	bAttrFlags := b&AttrBold | b&AttrUnderline | b&AttrReverse
	bColor := b &^ (AttrBold | AttrReverse | AttrUnderline)

	retColor := bColor
	if bColor == ColorDefault {
		retColor = aColor
	}

	return retColor | aAttrFlags | bAttrFlags
}
