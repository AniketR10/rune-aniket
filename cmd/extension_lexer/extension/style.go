package extension

import (
	"github.com/alecthomas/chroma"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/term"
)

// emulates extension_lsp semantic tokens style
func newDefaultStyle() *chroma.Style {
	builder := chroma.NewStyleBuilder("six")
	builder.AddEntry(chroma.Keyword, chroma.StyleEntry{
		Colour: chroma.NewColour(0xff, 0xff, 0),
	})
	builder.AddEntry(chroma.NameBuiltin, chroma.StyleEntry{
		Colour: chroma.NewColour(0xff, 0xff, 0),
	})
	builder.AddEntry(chroma.NameKeyword, chroma.StyleEntry{
		Colour: chroma.NewColour(0xff, 0xff, 0),
	})
	builder.AddEntry(chroma.LiteralString, chroma.StyleEntry{
		Colour: chroma.NewColour(0xff, 0, 0xff),
	})
	builder.AddEntry(chroma.LiteralNumber, chroma.StyleEntry{
		Colour: chroma.NewColour(0xff, 0, 0),
	})
	builder.AddEntry(chroma.Comment, chroma.StyleEntry{
		Colour: chroma.NewColour(0, 0, 0xff),
	})
	builder.AddEntry(chroma.CommentPreproc, chroma.StyleEntry{
		Colour: chroma.NewColour(0, 0, 0xff),
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
	var bgColor tcell.Color
	if !bg.IsZero() {
		bgColor = tcell.NewColor(
			int32(bg.Background.Red()),
			int32(bg.Background.Green()),
			int32(bg.Background.Blue()),
		)
	}
	for t := range chroma.StandardTypes {
		entry := style.Get(t)
		if t != chroma.Background {
			entry = entry.Sub(bg)
		}
		if entry.IsZero() {
			continue
		}
		converted[t] = styleEntryToAttr(setBackgroundAttr, bgColor, entry)
	}
	return converted
}

func styleEntryToAttr(setBackgroundAttr bool, bgColor tcell.Color, e chroma.StyleEntry) (
	ret term.Attributes,
) {
	ret.Fg = tcell.NewColor(
		int32(e.Colour.Red()),
		int32(e.Colour.Green()),
		int32(e.Colour.Blue()),
	)

	if setBackgroundAttr {
		ret.Bg = bgColor
	}
	if e.Bold == chroma.Yes {
		ret.Attrs |= tcell.AttrBold
	}
	if e.Underline == chroma.Yes {
		ret.Attrs |= tcell.AttrUnderline
	}
	if e.Italic == chroma.Yes {
		ret.Attrs |= tcell.AttrItalic
	}
	return ret
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
