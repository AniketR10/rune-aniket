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
					assert.Equal(t, 4, b.Height(20))
					b.Resize(20, 1)
				}, `
hello world         
                    
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() {
					b.Resize(20, 9)
					writeBuffer(b, ". Let's test its responsiveness")
					require.Equal(t, 6, b.Height(20))
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
					require.Equal(t, 8, b.Height(20))
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
					assert.Equal(t, 9, b.Height(20))
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
					assert.Equal(t, 11, b.Height(20))
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
				func() { b.Resize(2, 2) }, `
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
