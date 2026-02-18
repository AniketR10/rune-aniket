// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package markdown

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// Config defines the styling options for markdown rendering.
type Config struct {
	// Header styles (H1-H6)
	H1, H2, H3, H4, H5, H6 term.Attributes
	// HeaderPrefix controls whether headers display the '#' prefix (e.g., "# Title").
	// When true (default), headers show the markdown prefix to indicate nesting level.
	HeaderPrefix bool

	// Text styles
	Paragraph     term.Attributes
	Bold          term.Attributes
	Italic        term.Attributes
	Strikethrough term.Attributes

	// Code styles
	CodeBlock  term.Attributes
	InlineCode term.Attributes

	// Link styles
	Link    term.Attributes
	LinkURL term.Attributes

	// Blockquote styling
	Blockquote       term.Attributes
	BlockquoteBorder rune

	// List styling
	ListBullet        rune
	TaskListChecked   rune
	TaskListUnchecked rune

	// Table styling
	TableCharSet TableCharSet

	// Horizontal rule styling
	HorizontalRule     rune
	HorizontalRuleAttr term.Attributes

	// Spacing
	ParagraphSpacing int
}

// TableCharSet defines the characters used to draw table borders.
type TableCharSet struct {
	component.FrameUnionCharSet
	HeaderSeparator rune
	ColumnSeparator rune
	CrossJoin       rune
	TopJoin         rune
	BottomJoin      rune
}

// DefaultTableCharSet returns the default table character set.
func DefaultTableCharSet() TableCharSet {
	return TableCharSet{
		FrameUnionCharSet: component.DefaultFrameUnionCharSet(),
		HeaderSeparator:   '─',
		ColumnSeparator:   '│',
		CrossJoin:         '┼',
		TopJoin:           '┬',
		BottomJoin:        '┴',
	}
}

// DefaultConfig returns a Config with sensible defaults for terminal rendering.
func DefaultConfig() Config {
	magenta := term.Attributes{Fg: tcell.ColorFuchsia}
	magentaBold := term.Attributes{Fg: tcell.ColorFuchsia, Attrs: tcell.AttrBold}
	cyan := term.Attributes{Fg: tcell.ColorTeal}
	cyanUnderline := term.Attributes{Fg: tcell.ColorTeal, Attrs: tcell.AttrUnderline}
	codeblock := term.Attributes{Fg: tcell.ColorSilver, Bg: tcell.ColorGray}
	gray := term.Attributes{Fg: tcell.ColorGray}
	def := term.Attributes{Fg: tcell.ColorDefault}
	defBold := term.Attributes{Fg: tcell.ColorDefault, Attrs: tcell.AttrBold}
	defItalic := term.Attributes{Fg: tcell.ColorDefault, Attrs: tcell.AttrItalic}
	dimWhite := term.Attributes{Fg: tcell.ColorDimGray}
	dimWhiteStrike := term.Attributes{
		Fg: tcell.ColorDefault, Attrs: tcell.AttrStrikeThrough | tcell.AttrDim,
	}
	purpleBold := term.Attributes{Fg: tcell.ColorPurple, Attrs: tcell.AttrBold}
	purple := term.Attributes{Fg: tcell.ColorPurple}
	purpleDim := term.Attributes{Fg: tcell.ColorPurple, Attrs: tcell.AttrDim}

	title := term.Attributes{
		Fg:    tcell.ColorWhite,
		Bg:    tcell.ColorPurple,
		Attrs: tcell.AttrBold,
	}
	return Config{
		H1:           title,
		H2:           magentaBold,
		H3:           magenta,
		H4:           purpleBold,
		H5:           purple,
		H6:           purpleDim,
		HeaderPrefix: true,

		Paragraph:     def,
		Bold:          defBold,
		Italic:        defItalic,
		Strikethrough: dimWhiteStrike,

		CodeBlock:  codeblock,
		InlineCode: codeblock,

		Link:    cyanUnderline,
		LinkURL: cyan,

		Blockquote:       dimWhite,
		BlockquoteBorder: '│',

		ListBullet:        '•',
		TaskListChecked:   '☑',
		TaskListUnchecked: '☐',

		TableCharSet: DefaultTableCharSet(),

		HorizontalRule:     '─',
		HorizontalRuleAttr: gray,

		ParagraphSpacing: 1,
	}
}
