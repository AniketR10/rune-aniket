package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestContainerDraw(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		l := NewContainer()
		l.Resize(4, 4)
		w := term.NewStringWriter(20, 9)
		tests := []testutil.ComponentTestCase{
			{
				nil, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
			},
		}
		testutil.TestComponent(t, l, w, tests)
	})

	t.Run("happy path", func(t *testing.T) {
		l := NewContainer()
		r1 := l.AddRow()
		r1.AddComponent(testResponsive('a'), 6)
		r1.AddComponent(testResponsive('b'), 6)
		r2 := l.AddRow()
		r2.AddComponent(testResponsive('c'), 6)
		r2.AddComponent(testResponsive('d'), 6)
		r3 := l.AddRow()
		r3.AddComponent(testResponsive('e'), 6)
		r3.AddComponent(testResponsive('f'), 6)
		l.Resize(20, 4)

		w := term.NewStringWriter(20, 9)

		tests := []testutil.ComponentTestCase{
			{
				nil, `
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { l.Resize(20, 9) }, `
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { l.Resize(2, 2) }, `
ab                  
cd                  
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { require.False(t, l.ScrollUp()) }, `
ab                  
cd                  
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { require.True(t, l.ScrollDown()) }, `
cd                  
ef                  
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { require.True(t, l.ScrollDown()) }, `
ef                  
                    
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() { require.False(t, l.ScrollDown()) }, `
ef                  
                    
                    
                    
                    
                    
                    
                    
                    `,
			}, {
				func() {
					for l.ScrollUp() {
					}
					l.Resize(20, 9)
					row := l.AddRow()
					row.AddComponent(testResponsive('Z'), 3)
					row.AddComponent(testResponsive('Y'), 6)
					row.AddComponent(testResponsive('X'), 3)
				}, `
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
ZZZZZYYYYYYYYYYXXXXX
                    
                    
                    
                    
                    `,
			},
		}
		testutil.TestComponent(t, l, w, tests)
	})

	t.Run("zero row", func(t *testing.T) {
		l := NewContainer()
		r1 := l.AddRow()
		r1.AddComponent(testResponsive('a'), 12)
		_ = l.AddRow()
		r3 := l.AddRow()
		r3.AddComponent(testResponsive('b'), 12)
		l.Resize(4, 4)
		w := term.NewStringWriter(20, 9)
		tests := []testutil.ComponentTestCase{
			{
				nil, `
aaaa                
bbbb                
                    
                    
                    
                    
                    
                    
                    `,
			},
		}
		testutil.TestComponent(t, l, w, tests)
	})

}

func testResponsive(ch rune) Responsive {
	return &TestResponsive{
		WantHeight: 1,
		TestComponent: TestComponent{
			Ch: ch,
		},
	}
}
