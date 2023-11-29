package input

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestBox(t *testing.T) {
	t.Run("min and max height passed are coherent or else it panics", func(t *testing.T) {
		// ok
		NewBox(cell.NewBuffer(), BoxConfig{})

		// ok
		NewBox(cell.NewBuffer(), BoxConfig{MinHeight: 1})

		// ok
		NewBox(cell.NewBuffer(), BoxConfig{MaxHeight: 1})

		// ok
		NewBox(cell.NewBuffer(), BoxConfig{MaxHeight: 1, MinHeight: 1})

		// ok
		NewBox(cell.NewBuffer(), BoxConfig{MaxHeight: 2, MinHeight: 1})

		assert.Panics(t, func() {
			NewBox(cell.NewBuffer(), BoxConfig{MaxHeight: 1, MinHeight: 3})
		})
	})
	t.Run("no placeholder", func(t *testing.T) {
		buf := cell.NewBuffer()
		b := NewBox(buf, BoxConfig{})
		b.Resize(20, 4)

		w := term.NewStringWriter(20, 9)

		tests := []testutil.ComponentTestCase{
			{
				nil, `
┌──────────────────┐
│                  │
│                  │
└──────────────────┘
                    
                    
                    
                    
                    `,
			}, {
				func() { b.Resize(2, 2) }, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { b.Resize(20, 9) }, `
┌──────────────────┐
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() { b.Resize(20, 1) }, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { b.Resize(20, 9) }, `
┌──────────────────┐
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() {
					writeBuffer(b, "hello world")
					assert.Equal(t, 3, b.Height(20))
					b.Resize(20, 1)
				}, `
hello world         
                    
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() {
					b.Resize(20, 9)
					writeBuffer(b, ". Let's test its responsiveness")
					require.Equal(t, 5, b.Height(20))
					b.Resize(20, 6)
				}, `
┌──────────────────┐
│hello world. Let's│
│ test its responsi│
│veness            │
│                  │
└──────────────────┘
                    
                    
                    `,
			}, {
				func() {
					writeBuffer(b, ". Let's test its responsiveness")
					require.Equal(t, 7, b.Height(20))
					b.Resize(20, 8)
				}, `
┌──────────────────┐
│hello world. Let's│
│ test its responsi│
│veness. Let's test│
│ its responsivenes│
│s                 │
│                  │
└──────────────────┘
                    `,
			}, {
				func() {
					writeBuffer(b, ". Let's test its responsiveness")
					assert.Equal(t, 8, b.Height(20))
					b.Resize(20, 9)
				}, `
┌──────────────────┐
│hello world. Let's│
│ test its responsi│
│veness. Let's test│
│ its responsivenes│
│s. Let's test its │
│responsiveness    │
│                  │
└──────────────────┘`,
			}, {
				func() {
					writeBuffer(b, ". Let's test its scrolling")
					assert.Equal(t, 10, b.Height(20))
				}, `
┌──────────────────┐
│ test its responsi│
│veness. Let's test│
│ its responsivenes│
│s. Let's test its │
│responsiveness. Le│
│t's test its scrol│
│ling              │
└──────────────────┘`,
			}, {
				func() {
					handled := true
					for handled {
						_, handled = b.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
					}
				}, `
┌──────────────────┐
│hello world. Let's│
│ test its responsi│
│veness. Let's test│
│ its responsivenes│
│s. Let's test its │
│responsiveness. Le│
│t's test its scrol│
└──────────────────┘`,
			},
		}
		testutil.TestComponent(t, b, w, tests)
	})
	t.Run("with placeholder", func(t *testing.T) {
		buf := cell.NewBuffer()
		b := NewBox(buf, BoxConfig{Placeholder: "HERE..."})
		b.Resize(20, 4)

		w := term.NewStringWriter(20, 9)

		tests := []testutil.ComponentTestCase{
			{
				nil, `
┌──────────────────┐
│HERE...           │
│                  │
└──────────────────┘
                    
                    
                    
                    
                    `,
			}, {
				func() {
					assert.Equal(t, 3, b.Height(20))
					b.Resize(2, 2)
					assert.Equal(t, 3, b.Height(20))
				}, `
HE                  
RE                  
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { b.Resize(10, 2) }, `
HERE...             
                    
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { b.Resize(20, 9) }, `
┌──────────────────┐
│HERE...           │
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() { b.Resize(20, 9) }, `
┌──────────────────┐
│HERE...           │
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() {
					writeBuffer(b, "hello world")
					assert.Equal(t, 3, b.Height(20))
				}, `
┌──────────────────┐
│hello world       │
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() {
					// last row full feature
					writeBuffer(b, "xxxxxxx")
					assert.Equal(t, 4, b.Height(20))
				}, `
┌──────────────────┐
│hello worldxxxxxxx│
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() {
					writeBuffer(b, "x")
					assert.Equal(t, 4, b.Height(20))
				}, `
┌──────────────────┐
│hello worldxxxxxxx│
│x                 │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			},
		}
		testutil.TestComponent(t, b, w, tests)
	})

	t.Run("with long placeholder", func(t *testing.T) {
		buf := cell.NewBuffer()
		b := NewBox(buf, BoxConfig{Placeholder: "please write to your great, lovely, assistant"})
		b.Resize(20, 4)

		w := term.NewStringWriter(20, 9)

		tests := []testutil.ComponentTestCase{
			{
				nil, `
┌──────────────────┐
│please write to yo│
│ur great, lovely, │
└──────────────────┘
                    
                    
                    
                    
                    `,
			}, {
				func() {
					assert.Equal(t, 5, b.Height(20))
					b.Resize(2, 2)
					assert.Equal(t, 5, b.Height(20))
				}, `
pl                  
ea                  
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { b.Resize(10, 2) }, `
please wri          
te to your          
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { b.Resize(20, 9) }, `
┌──────────────────┐
│please write to yo│
│ur great, lovely, │
│assistant         │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() {
					writeBuffer(b, "hello world")
					assert.Equal(t, 3, b.Height(20))
				}, `
┌──────────────────┐
│hello world       │
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			},
		}
		testutil.TestComponent(t, b, w, tests)
	})
	t.Run("with min, max height", func(t *testing.T) {
		buf := cell.NewBuffer()
		b := NewBox(buf, BoxConfig{MinHeight: 4, MaxHeight: 5, Placeholder: "HERE..."})
		b.Resize(20, 4)

		w := term.NewStringWriter(20, 9)

		tests := []testutil.ComponentTestCase{
			{
				nil, `
┌──────────────────┐
│HERE...           │
│                  │
└──────────────────┘
                    
                    
                    
                    
                    `,
			}, {
				func() {
					assert.Equal(t, 4, b.Height(20))
					b.Resize(2, 2)
					assert.Equal(t, 4, b.Height(20))
				}, `
HE                  
RE                  
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() {
					b.Resize(20, 9)
					writeBuffer(b, "hello world")
					assert.Equal(t, 4, b.Height(20))
				}, `
┌──────────────────┐
│hello world       │
│                  │
│                  │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() {
					writeBuffer(b, "hello worldhelloworldhelloworld")
					assert.Equal(t, 5, b.Height(20))
				}, `
┌──────────────────┐
│hello worldhello w│
│orldhelloworldhell│
│oworld            │
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			}, {
				func() {
					// last row full feature is also capped by maxHeight
					writeBuffer(b, "xxxxxxxxxxxx")
					assert.Equal(t, 5, b.Height(20))
				}, `
┌──────────────────┐
│hello worldhello w│
│orldhelloworldhell│
│oworldxxxxxxxxxxxx│
│                  │
│                  │
│                  │
│                  │
└──────────────────┘`,
			},
		}
		testutil.TestComponent(t, b, w, tests)
	})
}

// we could write to cell.Buffer directly
// but this is a more realistic way
func writeBuffer(b tui.Handler, str string) {
	for _, r := range str {
		b.Handle(term.Event{Type: term.EventKey, Ch: r})
		b.Draw(term.NewStringWriter(20, 9))
		// scroll needs to be drawn for wrap features to be correct
		// width and height here are not important
	}
}
