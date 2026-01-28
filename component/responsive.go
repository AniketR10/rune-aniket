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
	"math"

	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

// Responsive components implement a backpressure mechanism (Height) for
// aggregate components to dynamically resize children based on their contents.
// See Height for more details.
type Responsive interface {
	tui.Component
	// Height allows for children components to return a height hint
	// given a width so a parent component can compose accordingly.
	// The returned height can be overriden at the parent's discretion
	// (i.e. there's simply no height left on the screen)
	// so implementers should expect that on calls to Resize.
	Height(width int) int
}

// StringResponsiveConfig adds responsive-specific configuration to a StringConfig.
type StringResponsiveConfig struct {
	// NoSplitWords instructs the underlying string responsive component
	// to attempt to not split words in half when possible.
	NoSplitWords bool
	StringConfig
}

// NewResponsiveString allocates storage for a new ResponsiveString based on str and cfg.
func NewResponsiveString(str string, cfg StringResponsiveConfig) *ResponsiveString {
	return NewResponsiveStringFromCells(cell.StringToCells(str), cfg)
}

// NewResponsiveStringFromCells returns a Responsive implementation for a matrix of cells.
func NewResponsiveStringFromCells(cells [][]term.Cell, cfg StringResponsiveConfig) *ResponsiveString {
	ret := new(ResponsiveString)
	ret.Init(cells, cfg)
	return ret
}

// Buffer wraps a cell.Buffer and returns a tui.Component which satisfies
// Responsive. Note that this is not the most efficient implementation of tui.Component
// for a cell.Buffer. See component.Scroll for more details.
func Buffer(buf *cell.Buffer, cfg StringResponsiveConfig) Responsive {
	return &respBuf{buf: buf, ResponsiveString: ResponsiveString{cfg: cfg}}
}

// NopResponsive returns a Responsive tui.Component that draws nothing.
func NopResponsive() Responsive {
	return nopResponsive{Component: Nop()}
}

// FuncResponsive wraps a tui.Component that satisfies Responsive's Height
// by calling heightFn.
func FuncResponsive(c tui.Component, heightFn func(width int) int) Responsive {
	return respFn{Component: c, heightFn: heightFn}
}

type nopResponsive struct {
	tui.Component
}

func (f nopResponsive) Height(width int) int {
	return 0
}

type respFn struct {
	tui.Component
	heightFn func(int) int
}

func (f respFn) Height(width int) int {
	return f.heightFn(width)
}

// ResponsiveString is a String component that also satisfies Responsive.
type ResponsiveString struct {
	cfg    StringResponsiveConfig
	in     [][]term.Cell
	out    floatingWithAttributes
	width  int
	height int
}

var _ WithAttributes = (*ResponsiveString)(nil)
var _ Scrollable = (*ResponsiveString)(nil)
var _ Responsive = (*ResponsiveString)(nil)
var _ Floating = (*ResponsiveString)(nil)
var _ fmt.Stringer = (*ResponsiveString)(nil)

var _ WithAttributes = (*respBuf)(nil)
var _ Scrollable = (*respBuf)(nil)
var _ Responsive = (*respBuf)(nil)
var _ fmt.Stringer = (*respBuf)(nil)

// Init initializes a ResponsiveString with the given cells and cfg.
func (s *ResponsiveString) Init(cells [][]term.Cell, cfg StringResponsiveConfig) {
	s.cfg = cfg
	s.in = cells
	s.Resize(0, 0) // initialize ret.out
}

// Height satisfies Responsive.
func (s *ResponsiveString) Height(width int) int {
	if width <= 0 {
		return 0
	}
	height := len(s.massageInput(width))
	height += s.cfg.PaddingVertical
	if s.cfg.FrameCharSet != (FrameCharSet{}) {
		height += 2
	}
	return height
}

// Dimensions returns the optimal width and height.
func (s *ResponsiveString) Dimensions() (width, height int) {
	return s.out.Dimensions()
}

// Resize satisfies tui.Component.
func (s *ResponsiveString) Resize(width, height int) {
	s.width = width
	s.height = height
	outRaw := s.massageInput(width)
	s.out = newStringComp(outRaw, s.cfg.Attributes, ' ',
		s.cfg.BackgroundAttributes, s.cfg.FrameCharSet,
		s.cfg.PaddingHorizontal, s.cfg.PaddingVertical, s.cfg.Alignment, s.cfg.MinWidth)
	s.out.Resize(width, height)
}

// Draw satisfies tui.Component.
func (s *ResponsiveString) Draw(w term.Writer) {
	s.out.Draw(w)
}

// SetAttr satisfies WithAttributes.
func (s *ResponsiveString) SetAttr(attr term.Attributes) term.Attributes {
	s.cfg.Attributes = attr
	// do not set s.cfg.BackgroundAttributes
	// as this is not what the user most likely intends.
	return s.out.SetAttr(attr)
}

// SeekUp always returns false.
func (s *ResponsiveString) SeekUp() bool {
	return false
}

// SeekDown always returns false.
func (s *ResponsiveString) SeekDown() bool {
	return false
}

// SeekOffset always returns 0.
func (s *ResponsiveString) SeekOffset() int {
	return 0
}

// MaxSeekOffset always returns 0.
func (s *ResponsiveString) MaxSeekOffset() int {
	return 0
}

// String satisfies fmt.Stringer.
func (s *ResponsiveString) String() string {
	return s.out.String()
}

func (s *ResponsiveString) massageInput(width int) [][]term.Cell {
	effectiveWidth := width
	if effectiveWidth > 2 && s.cfg.FrameCharSet != (FrameCharSet{}) {
		effectiveWidth -= 2
	}
	if effectiveWidth > s.cfg.PaddingHorizontal {
		effectiveWidth -= s.cfg.PaddingHorizontal
	}
	var outRaw [][]term.Cell
	for _, col := range s.in {
		if len(col) == 0 {
			outRaw = append(outRaw, col[:])
			continue
		}
		for len(col) > 0 {
			chunkLen := int(math.Min(float64(len(col)), float64(effectiveWidth)))
			if chunkLen == 0 {
				break
			}
			origChunkLen := chunkLen
			// do not split word in half
			for s.cfg.NoSplitWords && origChunkLen != len(col) && chunkLen > 1 && col[chunkLen-1].Ch != ' ' {
				chunkLen--
			}
			// word doesn't fit, split word
			if chunkLen == 1 {
				chunkLen = origChunkLen
			}
			outRaw = append(outRaw, col[:chunkLen])
			col = col[chunkLen:]
		}
	}
	return outRaw
}

type respBuf struct {
	ResponsiveString
	buf           *cell.Buffer
	width, height int
}

func (b *respBuf) Height(width int) int {
	b.ResponsiveString.in = b.buf.RawCells()
	return b.ResponsiveString.Height(width)
}

func (b *respBuf) Resize(width, height int) {
	b.width, b.height = width, height
}

func (b *respBuf) Draw(w term.Writer) {
	b.ResponsiveString.in = b.buf.RawCells()
	b.ResponsiveString.Resize(b.width, b.height)
	b.ResponsiveString.Draw(w)
}
