package component

import (
	"testing"

	"github.com/ernestrc/fractal/term"
)

func TestDrawDefaultSpan(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	s := NewSpan(u)

	s.Resize(8, 4)

	w := term.NewStringWriter(9, 5)

	tests := []testCase{
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
			func() { s.Padding.HorizontalPerc, s.Padding.VerticalPerc = 0.5, 0.5; s.Resize(8, 4) }, `
****     
****     
         
         
         `,
		}, {
			func() { s.Resize(4, 4) }, `
**       
**       
         
         
         `,
		}, {
			func() { s.ContentAlignment = SpanAlignmentRight; s.Resize(8, 4) }, `
    **** 
    **** 
         
         
         `,
		}, {
			func() { s.ContentAlignment |= SpanAlignmentBottom; s.Resize(8, 4) }, `
         
         
    **** 
    **** 
         `,
		}, {
			func() { s.ContentAlignment = SpanAlignmentLeft | SpanAlignmentBottom; s.Resize(8, 4) }, `
         
         
****     
****     
         `,
		}, {
			func() {
				s.ContentAlignment = SpanAlignmentHorizontallyCentered | SpanAlignmentBottom
				s.Resize(8, 4)
			}, `
         
         
  ****   
  ****   
         `,
		}, {
			func() {
				s.ContentAlignment = SpanAlignmentHorizontallyCentered | SpanAlignmentTop
				s.Resize(8, 4)
			}, `
  ****   
  ****   
         
         
         `,
		}, {
			func() {
				s.ContentAlignment = SpanAlignmentHorizontallyCentered | SpanAlignmentVerticallyCentered
				s.Resize(8, 4)
			}, `
         
  ****   
  ****   
         
         `,
		}, {
			func() {
				s.ContentAlignment = SpanAlignmentLeft | SpanAlignmentVerticallyCentered
				s.Resize(8, 4)
			}, `
         
****     
****     
         
         `,
		}, {
			func() {
				s.ContentAlignment = SpanAlignmentRight | SpanAlignmentVerticallyCentered
				s.Resize(8, 4)
			}, `
         
    **** 
    **** 
         
         `,
		}, {
			/* padding is removed if dimensions are < 3 */
			func() {
				s.ContentAlignment = SpanAlignmentRight | SpanAlignmentVerticallyCentered
				s.Resize(2, 2)
			}, `
**       
**       
         
         
         `,
		}, {
			func() {
				s.ContentAlignment = SpanAlignmentCentered

				// Vertical/Horizontal override Perc
				s.Padding.Vertical = 2
				s.Padding.Horizontal = 4

				s.Resize(8, 4)
			}, `
         
  ****   
  ****   
         
         `,
		}, {
			func() {
				s.ContentAlignment = SpanAlignmentCentered

				// Vertical/Horizontal negatie is used
				// as effective content width/height
				s.Padding.Vertical = -4
				s.Padding.Horizontal = -6

				s.Resize(8, 4)
			}, `
 ******  
 ******  
 ******  
 ******  
         `,
		},
	}

	testWorkflow(t, s, w, tests)
}
