package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestDrawFrame(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	f := NewFrame(u)

	f.Resize(8, 4)

	w := term.NewStringWriter(9, 5)

	tests := []testutil.ComponentTestCase{
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

	testutil.TestComponent(t, f, w, tests)
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
