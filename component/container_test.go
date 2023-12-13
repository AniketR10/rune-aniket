package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestContainerDimensions(t *testing.T) {
	l := NewContainer()
	l.Resize(4, 4)

	row1 := l.AddRow()
	row1.AddComponent(testResponsive('a', 2), MaxCols)

	row2 := l.AddRow()
	row2.AddComponent(testResponsive('a', 3), MaxCols/2)
	row2.AddComponent(testResponsive('a', 3), MaxCols/2)

	width, height := l.Dimensions()
	assert.Equal(t, 6, width)
	assert.Equal(t, 5, height)
}

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
		a := testResponsive('a', 1)
		r1.AddComponent(a, 6)
		r1.AddComponent(testResponsive('b', 1), 6)
		r2 := l.AddRow()
		r2.AddComponent(testResponsive('c', 1), 6)
		r2.AddComponent(testResponsive('d', 1), 6)
		r3 := l.AddRow()
		r3.AddComponent(testResponsive('e', 1), 6)
		r3.AddComponent(testResponsive('f', 1), 6)
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
					row.AddComponent(testResponsive('Z', 1), 3)
					row.AddComponent(testResponsive('Y', 1), 6)
					row.AddComponent(testResponsive('X', 1), 3)
				}, `
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
ZZZZZYYYYYYYYYYXXXXX
                    
                    
                    
                    
                    `,
			}, {
				func() {
					row := l.AddRow()
					row.AddComponent(testResponsive('3', 3), 3)
					row.AddComponent(testResponsive('2', 2), 3)
					row.AddComponent(testResponsive('1', 1), 3)
				}, `
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
ZZZZZYYYYYYYYYYXXXXX
333332222211111     
333332222211111     
333332222211111     
                    
                    `,
			}, {
				func() {
					// changing height requirements without container
					// or row "knowing" about it
					a.(*TestResponsive).WantHeight = 2
				}, `
aaaaaaaaaabbbbbbbbbb
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
ZZZZZYYYYYYYYYYXXXXX
333332222211111     
333332222211111     
333332222211111     
                    `,
			}, {
				func() {
					a.(*TestResponsive).WantHeight = 4
				}, `
aaaaaaaaaabbbbbbbbbb
aaaaaaaaaabbbbbbbbbb
aaaaaaaaaabbbbbbbbbb
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
ZZZZZYYYYYYYYYYXXXXX
333332222211111     
333332222211111     `,
			}, {
				func() {
					require.True(t, l.ScrollDown())
				}, `
aaaaaaaaaabbbbbbbbbb
aaaaaaaaaabbbbbbbbbb
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
ZZZZZYYYYYYYYYYXXXXX
333332222211111     
333332222211111     
333332222211111     `,
			}, {
				func() {
					for l.ScrollDown() {
					}
					row := l.AddRow()
					row.AddComponent(testResponsive('@', 3), 6)
					row.AddComponent(testResponsive('#', 2), 5)
					row.AddComponent(testResponsive('$', 1), 5)
				}, `
333332222211111     
@@@@@@@@@@########$$
@@@@@@@@@@########$$
@@@@@@@@@@########$$
                    
                    
                    
                    
                    `,
			},
		}
		testutil.TestComponent(t, l, w, tests)
	})

	t.Run("zero row", func(t *testing.T) {
		l := NewContainer()
		r1 := l.AddRow()
		r1.AddComponent(testResponsive('a', 1), 12)
		_ = l.AddRow()
		r3 := l.AddRow()
		r3.AddComponent(testResponsive('b', 1), 12)
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

func testResponsive(ch rune, wantHeight int) Responsive {
	return &TestResponsive{
		WantWidth:  wantHeight,
		WantHeight: wantHeight,
		TestComponent: TestComponent{
			Ch: ch,
		},
	}
}
