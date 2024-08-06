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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestSpanDimensions(t *testing.T) {
	t.Run("panics if underlying component is not Floating", func(t *testing.T) {
		assert.Panics(t, func() {
			comp := &TestComponent{}
			s := NewSpan(comp, SpanConfig{})
			s.Dimensions()
		})
	})
	t.Run("uses underlying Floating dimensions if padding is 0", func(t *testing.T) {
		comp := StaticFloating(&TestComponent{}, 10, 20)
		s := NewSpan(comp, SpanConfig{})
		width, height := s.Dimensions()
		assert.Equal(t, 10, width)
		assert.Equal(t, 20, height)
	})
	t.Run("adds absolute vertical padding from underlying component's returned Dimensions", func(t *testing.T) {
		comp := StaticFloating(&TestComponent{}, 10, 20)
		s := NewSpan(comp, SpanConfig{PadVertical: 2})
		width, height := s.Dimensions()
		assert.Equal(t, 10, width)
		assert.Equal(t, 22, height)
	})
	t.Run("subtracts absolute horizontal padding from underlying component's call to Dimensions", func(t *testing.T) {
		comp := StaticFloating(&TestComponent{}, 10, 20)
		s := NewSpan(comp, SpanConfig{PadHorizontal: 2})
		width, height := s.Dimensions()
		assert.Equal(t, 12, width)
		assert.Equal(t, 20, height)
	})
	t.Run("adds perc vertical padding from underlying component's returned Dimensions", func(t *testing.T) {
		comp := StaticFloating(&TestComponent{}, 10, 20)
		s := NewSpan(comp, SpanConfig{PadVerticalPerc: 0.2})
		// needs Resize or else it does vertical perc over 0
		s.Resize(10, 10)
		width, height := s.Dimensions()
		assert.Equal(t, 10, width)
		assert.Equal(t, 22, height)
	})
	t.Run("subtracts perc horizontal padding from underlying component's call to Dimensions", func(t *testing.T) {
		comp := StaticFloating(&TestComponent{}, 10, 20)
		s := NewSpan(comp, SpanConfig{PadHorizontalPerc: 0.1})
		width, height := s.Dimensions()
		assert.Equal(t, 11, width)
		assert.Equal(t, 20, height)
	})
	t.Run("adds relative horizontal padding and passes remaining with to underlying component's Dimensions", func(t *testing.T) {
		comp := StaticFloating(&TestComponent{}, 10, 20)
		s := NewSpan(comp, SpanConfig{PadHorizontal: -2})
		s.Resize(20, 20)
		width, height := s.Dimensions()
		assert.Equal(t, 18, width)
		assert.Equal(t, 20, height)
	})
}

func TestSpanHeight(t *testing.T) {
	t.Run("panics if underlying component is not Responsive", func(t *testing.T) {
		assert.Panics(t, func() {
			comp := &TestComponent{}
			s := NewSpan(comp, SpanConfig{})
			s.Height(1)
		})
	})
	t.Run("uses underlying Responsive Height if padding is 0", func(t *testing.T) {
		comp := &TestResponsive{WantHeight: 10}
		s := NewSpan(comp, SpanConfig{})
		assert.Equal(t, 10, s.Height(0))
	})
	t.Run("adds absolute vertical padding from underlying component's returned Height", func(t *testing.T) {
		comp := &TestResponsive{WantHeight: 10}
		s := NewSpan(comp, SpanConfig{PadVertical: 2})
		assert.Equal(t, 12, s.Height(0))
	})
	t.Run("subtracts absolute horizontal padding from underlying component's call to Height", func(t *testing.T) {
		comp := &TestResponsive{WantHeight: 10}
		s := NewSpan(comp, SpanConfig{PadHorizontal: 2})
		require.Equal(t, 10, s.Height(20))
		assert.Equal(t, 18, comp.PassedWidth)
	})
	t.Run("adds perc vertical padding from underlying component's returned Height", func(t *testing.T) {
		comp := &TestResponsive{WantHeight: 10}
		s := NewSpan(comp, SpanConfig{PadVerticalPerc: 0.2})
		// needs Resize or else it does vertical perc over 0
		s.Resize(10, 10)
		assert.Equal(t, 12, s.Height(0))
	})
	t.Run("subtracts perc horizontal padding from underlying component's call to Height", func(t *testing.T) {
		comp := &TestResponsive{WantHeight: 10}
		s := NewSpan(comp, SpanConfig{PadHorizontalPerc: 0.1})
		require.Equal(t, 10, s.Height(20))
		assert.Equal(t, 18, comp.PassedWidth)
	})
	t.Run("ignores relative PadVertical when underlying component is Responsive", func(t *testing.T) {
		comp := &TestResponsive{WantHeight: 10}
		s := NewSpan(comp, SpanConfig{PadVertical: -2})
		s.Resize(10, 10)
		assert.Equal(t, 10, s.Height(0))
	})
	t.Run("adds relative horizontal padding and passes remaining with to underlying component's Height", func(t *testing.T) {
		comp := &TestResponsive{WantHeight: 10}
		s := NewSpan(comp, SpanConfig{PadHorizontal: -2})
		s.Resize(20, 20)
		require.Equal(t, 10, s.Height(20))
		assert.Equal(t, 2, comp.PassedWidth)
	})
}

func TestDrawDefaultSpan(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	cfg := DefaultSpanConfig()
	s := NewSpan(u, cfg)

	s.Resize(8, 4)

	w := term.NewStringWriter(9, 5)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
         `,
		}, {
			func() { s.SetContent(&TestComponent{Ch: '*'}) }, `
******** 
******** 
******** 
******** 
         `,
		}, {
			func() { s.cfg.PadHorizontalPerc, s.cfg.PadVerticalPerc = 0.5, 0.5; s.Resize(8, 4) }, `
         
  ****   
  ****   
         
         `,
		}, {
			func() { s.Resize(4, 4) }, `
         
 **      
 **      
         
         `,
		}, {
			func() { s.cfg.ContentAlignment = SpanAlignmentRight; s.Resize(8, 4) }, `
    **** 
    **** 
         
         
         `,
		}, {
			func() { s.cfg.ContentAlignment |= SpanAlignmentBottom; s.Resize(8, 4) }, `
         
         
    **** 
    **** 
         `,
		}, {
			func() { s.cfg.ContentAlignment = SpanAlignmentLeft | SpanAlignmentBottom; s.Resize(8, 4) }, `
         
         
****     
****     
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentHorizontallyCentered | SpanAlignmentBottom
				s.Resize(8, 4)
			}, `
         
         
  ****   
  ****   
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentHorizontallyCentered | SpanAlignmentTop
				s.Resize(8, 4)
			}, `
  ****   
  ****   
         
         
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentHorizontallyCentered | SpanAlignmentVerticallyCentered
				s.Resize(8, 4)
			}, `
         
  ****   
  ****   
         
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentLeft | SpanAlignmentVerticallyCentered
				s.Resize(8, 4)
			}, `
         
****     
****     
         
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentRight | SpanAlignmentVerticallyCentered
				s.Resize(8, 4)
			}, `
         
    **** 
    **** 
         
         `,
		}, {
			/* padding is removed if dimensions are < 3 */
			func() {
				s.cfg.ContentAlignment = SpanAlignmentRight | SpanAlignmentVerticallyCentered
				s.Resize(2, 2)
			}, `
**       
**       
         
         
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentCentered

				// Vertical/Horizontal override Perc
				s.cfg.PadVertical = 2
				s.cfg.PadHorizontal = 4

				s.Resize(8, 4)
			}, `
         
  ****   
  ****   
         
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentCentered
				// Vertical/Horizontal greater than size
				s.cfg.PadVertical = 9
				s.cfg.PadHorizontal = 5
				s.Resize(8, 4)
			}, `
  ***    
  ***    
  ***    
  ***    
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentCentered

				// Vertical/Horizontal negatie is used
				// as effective content width/height
				s.cfg.PadVertical = -4
				s.cfg.PadHorizontal = -6

				s.Resize(8, 4)
			}, `
 ******  
 ******  
 ******  
 ******  
         `,
		}, {
			func() {
				s.cfg.ContentAlignment = SpanAlignmentCentered
				// bigger than available
				s.cfg.PadVertical = -5
				s.cfg.PadHorizontal = -9

				s.Resize(8, 4)
			}, `
******** 
******** 
******** 
******** 
         `,
		},
	}

	testutil.TestComponent(t, s, w, tests)
}
