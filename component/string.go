// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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
package component

import (
	"fmt"

	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

// StringConfig defines options for StringConfig and StringResponsive
// constructors.
type StringConfig struct {
	Alignment
	term.Attributes
	FrameCharSet
	BackgroundAttributes term.Attributes
	BackgroundRune       rune
	Tabspaces            int
	PaddingVertical      int
	PaddingHorizontal    int
	MinWidth             int
}

// String is a tui.Component that draws a string with or without
// a background. It satisfies both Floating and WithAttributes.
// See NewString and NewStringWithConfig for more details.
type String struct {
	cfg StringConfig
	floatingWithAttributes
}

var _ WithAttributes = String{}
var _ Floating = String{}
var _ fmt.Stringer = String{}

// NewStringWithConfig converts a string into a static tui.Compontent with
// background/foreground attributes, content alignment and a frame,
// all configurable through cfg. The returned component is significantly
// slower to Draw and Resize than the component returned by String.
func NewStringWithConfig(str string, cfg StringConfig) String {
	cells := cell.StringToCells(str, cfg.Tabspaces)
	comp := newStringComp(cells, cfg.Attributes, cfg.BackgroundRune,
		cfg.BackgroundAttributes, cfg.FrameCharSet,
		cfg.PaddingHorizontal, cfg.PaddingVertical, cfg.Alignment, cfg.MinWidth)
	return String{
		cfg:                    cfg,
		floatingWithAttributes: comp,
	}
}

// NewString converts a string into top left centered one line tui.Component
// which draws the given string. If the string needs to be centered dynamically,
// or drawn multi-line use StringWithConfig instead.
func NewString(str string) String {
	return NewStringWithConfig(str, StringConfig{
		Alignment: SpanAlignmentLeft,
	})
}

// Config returns this String's StringConfig.
func (s String) Config() StringConfig {
	return s.cfg
}

// LazyBytes is an immutable String component that is allocation free
// until the first call to Draw.
//
// Akin to String, it compacts string into one line and but it
// takes data as a slice of bytes and a set of x positions
// to apply TokenAttributes to. In contrast, SetAttr it's a O(n)
// rather than String's O(1), so do not call it for every string
// after they have been drawn once, otherwise just use String,
// which will offer better features and performance characteristics.
//
// It should be wrapped by a Virtual component or used with
// a term.Writer to handle out of bound calls to SetCell.
//
// It is useful for collections, where not all strings need to
// be drawn and there's a clear performance requirement
// that offsets its limitations.
//
// Note that grapheme clusters are not supported by this component.
type LazyBytes struct {
	Data            []byte
	Tokens          []int
	Attributes      term.Attributes
	TokenAttributes term.Attributes
	cells           []term.Cell
}

var _ WithAttributes = (*LazyBytes)(nil)

// Resize is ignored.
func (l *LazyBytes) Resize(width, height int) {
}

// SetAttr sets the default attributes of the next call to Draw.
// If Draw has already been called, then this method is force all
// cells to be re-computed, so it should be used with care.
func (l *LazyBytes) SetAttr(attr term.Attributes) (ret term.Attributes) {
	ret = l.Attributes
	l.Attributes = attr
	if l.cells != nil {
		l.build()
	}
	return ret
}

func (l *LazyBytes) build() {
	l.cells = make([]term.Cell, len(l.Data))
	for i, r := range l.Data {
		l.cells[i] = term.Cell{
			Ch:         rune(r),
			Attributes: l.Attributes,
			Width:      1, // only support width=1 graphemes
		}
	}
	for _, t := range l.Tokens {
		l.cells[t].Bg |= l.TokenAttributes.Bg
		l.cells[t].Fg |= l.TokenAttributes.Fg
	}
}

// Draw satisfies tui.Component.
func (l *LazyBytes) Draw(w term.Writer) {
	if l.cells == nil {
		l.build()
	}
	for x, c := range l.cells {
		w.SetCell(term.Coordinates{X: x, Y: 0}, c)
	}
}

type floatingWithAttributes interface {
	tui.Component
	Floating
	WithAttributes
	fmt.Stringer
}

type stringComp struct {
	width, height int
	cells         [][]term.Cell
	attr          term.Attributes
}

func (s *stringComp) String() string {
	return cell.CellsToString(s.cells)
}

func (s *stringComp) Resize(width, height int) {
	s.width, s.height = width, height
}

func (s *stringComp) Draw(w term.Writer) {
	for y, r := range s.cells {
		if y >= s.height {
			break
		}
		for x, c := range r {
			if x >= s.width {
				break
			}
			if c.Ch == 0 {
				continue
			}
			w.SetCell(term.Coordinates{X: x, Y: y},
				term.Cell{
					Attributes: s.attr,
					Ch:         c.Ch,
					Combining:  c.Combining,
					Width:      c.Width,
				})
		}
	}
}

func (s *stringComp) SetAttr(attr term.Attributes) (ret term.Attributes) {
	ret = s.attr
	s.attr = attr
	return
}

func (s *stringComp) Dimensions() (width, height int) {
	height = len(s.cells)
	for _, row := range s.cells {
		if col := len(row); col > width {
			width = col
		}
	}
	return
}

// used to wrap Background and provide SetAttr to underlying stringComp
type backgroundStrWrapper struct {
	frame bool
	pad   bool
	span  *Span
	*Background
}

func (s backgroundStrWrapper) SetAttr(attr term.Attributes) term.Attributes {
	return s.Background.SetAttr(attr)
}

func (s backgroundStrWrapper) String() string {
	spanContent := s.Background.Content().(*Span).Content()
	if s.frame {
		return spanContent.(stringerFrame).String()
	}
	return spanContent.(*stringComp).String()
}

func (s backgroundStrWrapper) Dimensions() (width, height int) {
	return s.span.Dimensions()
}

func withBackgroundWrapper(
	comp WithAttributes, height, width int, background term.Cell,
	frame, pad bool, alignment Alignment,
) backgroundStrWrapper {
	span := NewSpan(comp, SpanConfig{
		PadVertical:      -height,
		PadHorizontal:    -width,
		ContentAlignment: alignment,
	})
	return backgroundStrWrapper{
		frame:      frame,
		pad:        pad,
		span:       span,
		Background: WithBackground(span, background),
	}
}

func newStringComp(
	cells [][]term.Cell, attr term.Attributes, c rune, battr term.Attributes,
	frameCharSet FrameCharSet, padWidth, padHeight int, alg Alignment,
	minWidth int,
) floatingWithAttributes {
	var comp floatingWithAttributes

	comp = &stringComp{cells: cells, attr: attr}
	shouldFrame := frameCharSet != (FrameCharSet{})

	var height int
	width := minWidth
	if shouldFrame {
		width -= (2 + padWidth)
	}
	for _, row := range cells {
		if len(row) > width {
			width = len(row)
		}
	}

	background := term.Cell{Ch: c, Attributes: battr}
	height = len(cells)
	shouldPad := padWidth != 0 || padHeight != 0

	if shouldFrame {
		// if inner pad is provided, center text
		if shouldPad {
			// background of inner padding looks better if it's the same attr
			// as the text.
			background := term.Cell{Ch: c, Attributes: attr}
			comp = withBackgroundWrapper(comp, height, width, background, false, false, alg)
		}
		width += 2 + padWidth
		height += 2 + padHeight

		frame := NewFrame(comp)
		frame.Attributes = attr
		frame.FrameCharSet = frameCharSet
		comp = stringerFrame{Stringer: comp, Frame: frame}
	}

	return withBackgroundWrapper(comp, height, width, background, shouldFrame, shouldPad, alg)
}

type stringerFrame struct {
	fmt.Stringer
	*Frame
}
