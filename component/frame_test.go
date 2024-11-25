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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/term"
)

func TestDrawFrame(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	f := NewFrame(u)

	f.Resize(8, 4)

	w := term.NewStringWriter(9, 5)

	tests := []comptest.TestCase{
		{
			nil, `
┌──────┐ 
│XXXXXX│ 
│XXXXXX│ 
└──────┘ 
         `,
		}, {
			func() { f.SetContent(&TestComponent{Ch: '*'}) }, `
┌──────┐ 
│******│ 
│******│ 
└──────┘ 
         `,
		}, {
			func() { f.Resize(4, 4) }, `
┌──┐     
│**│     
│**│     
└──┘     
         `,
		}, {
			func() { f.SetContent(&TestComponent{Ch: 'T'}) }, `
┌──┐     
│TT│     
│TT│     
└──┘     
         `,
		}, {
			func() { f.Resize(2, 2) }, `
TT       
TT       
         
         
         `,
		}, {
			func() { f.Resize(8, 4) }, `
┌──────┐ 
│TTTTTT│ 
│TTTTTT│ 
└──────┘ 
         `,
		}, {
			func() {
				fb := FrameCharSetDefault()
				fb.HorizontalTop = '┄'
				fb.VerticalRight = '┊'
				f.FrameCharSet = fb
			}, `
┌┄┄┄┄┄┄┐ 
│TTTTTT┊ 
│TTTTTT┊ 
└──────┘ 
         `,
		}, {
			func() {
				f.FrameCharSet = FrameCharSetDefault()
				f.SetContent(NewString("123"))
				f.Resize(8, 1)
			}, `
123      
         
         
         
         `,
		},
	}

	comptest.TestComponent(t, f, w, tests)
}

func TestFrameDrawWithScrollable(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString("A\nB\nC\nD\nE\nF")
	u := NewScroll(buf)
	f := NewFrame(u)

	f.Resize(8, 4)
	f.ScrollBarChar = '|'

	w := term.NewStringWriter(9, 20)

	tests := []comptest.TestCase{
		{
			nil, `
┌──────┐ 
│A     | 
│B     │ 
└──────┘ 
         
         
         
         
         
         
         
         
         
         
         
         
         
         
         
         `,
		},
		{
			func() { f.Resize(8, 20) }, `
┌──────┐ 
│A     │ 
│B     │ 
│C     │ 
│D     │ 
│E     │ 
│F     │ 
│      │ 
│      │ 
│      │ 
│      │ 
│      │ 
│      │ 
│      │ 
│      │ 
│      │ 
│      │ 
│      │ 
│      │ 
└──────┘ `,
		},
		{
			func() { buf.WriteString("\n0\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12") }, `
┌──────┐ 
│A     | 
│B     | 
│C     | 
│D     | 
│E     | 
│F     | 
│0     | 
│1     | 
│2     | 
│3     | 
│4     | 
│5     | 
│6     | 
│7     | 
│8     | 
│9     | 
│10    | 
│11    | 
└──────┘ `,
		},
		{
			func() { buf.WriteString("\n13\n14\n15\n16\n17\n18\n19\n20") }, `
┌──────┐ 
│A     | 
│B     | 
│C     | 
│D     | 
│E     | 
│F     | 
│0     | 
│1     | 
│2     | 
│3     | 
│4     | 
│5     | 
│6     │ 
│7     │ 
│8     │ 
│9     │ 
│10    │ 
│11    │ 
└──────┘ `,
		},
		{
			func() { u.SeekDown() }, `
┌──────┐ 
│B     | 
│C     | 
│D     | 
│E     | 
│F     | 
│0     | 
│1     | 
│2     | 
│3     | 
│4     | 
│5     | 
│6     | 
│7     │ 
│8     │ 
│9     │ 
│10    │ 
│11    │ 
│12    │ 
└──────┘ `,
		},
		{
			func() { u.SeekDown() }, `
┌──────┐ 
│C     │ 
│D     | 
│E     | 
│F     | 
│0     | 
│1     | 
│2     | 
│3     | 
│4     | 
│5     | 
│6     | 
│7     | 
│8     | 
│9     │ 
│10    │ 
│11    │ 
│12    │ 
│13    │ 
└──────┘ `,
		},
		{
			func() { u.SeekEndFile() }, `
┌──────┐ 
│3     │ 
│4     │ 
│5     │ 
│6     │ 
│7     │ 
│8     │ 
│9     | 
│10    | 
│11    | 
│12    | 
│13    | 
│14    | 
│15    | 
│16    | 
│17    | 
│18    | 
│19    | 
│20    | 
└──────┘ `,
		},
	}

	comptest.TestComponent(t, f, w, tests)
}

func TestComponentDimensions(t *testing.T) {
	f := NewFrame(StaticFloating(&TestComponent{}, 2, 2))
	actualWidth, actualHeight := f.Dimensions()
	assert.Equal(t, 4, actualWidth)
	assert.Equal(t, 4, actualHeight)
}

func TestFrameResponsive(t *testing.T) {
	t.Run("adds frame height to content's Height", func(t *testing.T) {
		f := NewFrame(testResponsive('a', 10))
		assert.Equal(t, 12, f.Height(10))
	})
	t.Run("it's conistent with Resize with width < 3 behaviour", func(t *testing.T) {
		f := NewFrame(testResponsive('a', 10))
		assert.Equal(t, 10, f.Height(2))
	})
	t.Run("it's conistent with Resize with height < 3 behaviour", func(t *testing.T) {
		f := NewFrame(testResponsive('a', 0))
		assert.Equal(t, 3, f.Height(10))
	})
}

func TestFrameWithAttributes(t *testing.T) {
	f := NewFrame(WithAttrSetter(&TestComponent{Ch: 'a'}))
	f.SetAttr(term.Attributes{Fg: tcell.ColorRed, Bg: tcell.ColorGreen})

	assert.Equal(t, tcell.ColorGreen, f.Attributes.Bg)
	assert.Equal(t, tcell.ColorRed, f.Attributes.Fg)

	prev := f.Content().(WithAttributes).SetAttr(term.Attributes{})
	assert.Equal(t, tcell.ColorGreen, prev.Bg)
	assert.Equal(t, tcell.ColorRed, prev.Fg)
}

func TestFrameScrollbarCalculate(t *testing.T) {
	suite := []struct {
		offset, maxOffset, height      int
		expectedOffset, expectedHeight int
	}{
		{
			offset:         0,
			maxOffset:      0,
			height:         10,
			expectedOffset: 1,
			expectedHeight: 8,
		},
		{
			offset:         10,
			maxOffset:      10,
			height:         12,
			expectedOffset: 6,
			expectedHeight: 5,
		},
		{
			offset:         10,
			maxOffset:      20,
			height:         12,
			expectedOffset: 4,
			expectedHeight: 4,
		},
		{
			offset:         500,
			maxOffset:      1000,
			height:         12,
			expectedOffset: 5,
			expectedHeight: 1,
		},
	}

	for i, test := range suite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			actualOffset, actualHeight := calculateScrollBar(
				test.offset, test.maxOffset, test.height)
			assert.Equal(t, test.expectedOffset, actualOffset, "offset")
			assert.Equal(t, test.expectedHeight, actualHeight, "height")
		})
	}

	t.Run("doesn't panic if offset > maxOffset", func(t *testing.T) {
		assert.NotPanics(t, func() {
			calculateScrollBar(999, 1, 10)
		})
	})

	t.Run("doesn't panic if everything is 0", func(t *testing.T) {
		assert.NotPanics(t, func() {
			calculateScrollBar(0, 0, 0)
		})
	})
}
