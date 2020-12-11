package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
)

func TestDrawDefaultSpan(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	cfg := DefaultSpanConfig()
	s := NewSpan(u, cfg)

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
		},
	}

	testWorkflow(t, s, w, tests)
}
