package extension

import (
	"github.com/alecthomas/chroma"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/color"
)

// emulates extension_lsp semantic tokens style
func newDefaultStyle() *chroma.Style {
	builder := chroma.NewStyleBuilder("six")
	builder.AddEntry(chroma.Keyword, chroma.StyleEntry{
		Colour: chroma.NewColour(255, 255, 0),
	})
	builder.AddEntry(chroma.NameBuiltin, chroma.StyleEntry{
		Colour: chroma.NewColour(255, 255, 0),
	})
	builder.AddEntry(chroma.NameKeyword, chroma.StyleEntry{
		Colour: chroma.NewColour(255, 255, 0),
	})
	builder.AddEntry(chroma.LiteralString, chroma.StyleEntry{
		Colour: chroma.NewColour(255, 0, 255),
	})
	builder.AddEntry(chroma.LiteralNumber, chroma.StyleEntry{
		Colour: chroma.NewColour(255, 0, 0),
	})
	builder.AddEntry(chroma.Comment, chroma.StyleEntry{
		Colour: chroma.NewColour(0, 0, 255),
	})
	builder.AddEntry(chroma.CommentPreproc, chroma.StyleEntry{
		Colour: chroma.NewColour(0, 0, 255),
	})

	ret, err := builder.Build()
	if err != nil {
		// should never panic, as we're building with structs directly
		panic(err)
	}
	return ret
}

func styleToAttrMap(
	style *chroma.Style, setBackgroundAttr bool,
) map[chroma.TokenType]term.Attributes {
	converted := make(map[chroma.TokenType]term.Attributes)
	bg := style.Get(chroma.Background)
	var bgAttr term.Attribute
	if !bg.IsZero() {
		bgAttr = color.RGBToAttribute(bg.Background.Red(), bg.Background.Green(), bg.Background.Blue())
	}
	for t := range chroma.StandardTypes {
		entry := style.Get(t)
		if t != chroma.Background {
			entry = entry.Sub(bg)
		}
		if entry.IsZero() {
			continue
		}
		converted[t] = styleEntryToAttr(setBackgroundAttr, bgAttr, entry)
	}
	return converted
}

func styleEntryToAttr(setBackgroundAttr bool, bgAttr term.Attribute, e chroma.StyleEntry) term.Attributes {
	fg := color.RGBToAttribute(e.Colour.Red(), e.Colour.Green(), e.Colour.Blue())
	var bg term.Attribute
	if setBackgroundAttr {
		bg = bgAttr
	}
	if e.Bold == chroma.Yes {
		fg |= term.AttrBold
	}
	if e.Underline == chroma.Yes {
		fg |= term.AttrUnderline
	}
	return term.Attributes{
		Fg: fg,
		Bg: bg,
	}
}

func attrForToken(
	styles map[chroma.TokenType]term.Attributes, tt chroma.TokenType,
) term.Attributes {
	if _, ok := styles[tt]; !ok {
		tt = tt.SubCategory()
		if _, ok := styles[tt]; !ok {
			tt = tt.Category()
			if _, ok := styles[tt]; !ok {
				return term.Attributes{}
			}
		}
	}
	return styles[tt]
}
