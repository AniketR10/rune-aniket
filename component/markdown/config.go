// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.


package markdown

import (
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
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
	// Parser, when set, enables syntax highlighting inside fenced code blocks.
	Parser syntaxapi.Parser
	// ScheduleNextTick schedules a function to run on the next event-loop
	// tick. Highlighting runs asynchronously via this callback so that
	// Highlight I/O does not block construction. Defaults to a
	// synchronous call in DefaultConfig.
	ScheduleNextTick func(func()) bool

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

	// Search highlight styles
	SearchMatch   term.Attributes // non-current search matches
	SearchCurrent term.Attributes // current search match
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
	magenta := term.Attributes{Fg: term.ColorFuchsia}
	magentaBold := term.Attributes{Fg: term.ColorFuchsia, Attrs: term.AttrBold}
	cyan := term.Attributes{Fg: term.ColorTeal}
	cyanUnderline := term.Attributes{Fg: term.ColorTeal, Attrs: term.AttrUnderline}
	codeblock := term.Attributes{Bg: term.ColorGray}
	gray := term.Attributes{Fg: term.ColorGray}
	def := term.Attributes{Fg: term.ColorDefault}
	defBold := term.Attributes{Fg: term.ColorDefault, Attrs: term.AttrBold}
	defItalic := term.Attributes{Fg: term.ColorDefault, Attrs: term.AttrItalic}
	dimWhite := term.Attributes{Fg: term.GetColor("dimgray")}
	dimWhiteStrike := term.Attributes{
		Fg: term.ColorDefault, Attrs: term.AttrStrikeThrough | term.AttrDim,
	}
	purpleBold := term.Attributes{Fg: term.ColorPurple, Attrs: term.AttrBold}
	purple := term.Attributes{Fg: term.ColorPurple}
	purpleDim := term.Attributes{Fg: term.ColorPurple, Attrs: term.AttrDim}

	title := term.Attributes{
		Fg:    term.ColorWhite,
		Bg:    term.ColorPurple,
		Attrs: term.AttrBold,
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

		ScheduleNextTick: func(cb func()) bool { cb(); return true },

		ParagraphSpacing: 1,

		SearchMatch:   term.Attributes{Attrs: term.AttrReverse},
		SearchCurrent: term.Attributes{Bg: term.ColorYellow, Fg: term.ColorBlack},
	}
}
