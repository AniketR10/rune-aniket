package component

import (
	"testing"

	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestGridDraw(t *testing.T) {
	t.Run("zero matrix", func(t *testing.T) {
		l := Grid(nil)
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
		l := Grid([][]tui.Component{
			[]tui.Component{&TestComponent{Ch: 'a'}, &TestComponent{Ch: 'b'}},
			[]tui.Component{&TestComponent{Ch: 'c'}, &TestComponent{Ch: 'd'}},
			[]tui.Component{&TestComponent{Ch: 'e'}, &TestComponent{Ch: 'f'}},
		})
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
aaaaaaaaaabbbbbbbbbb
aaaaaaaaaabbbbbbbbbb
ccccccccccdddddddddd
ccccccccccdddddddddd
ccccccccccdddddddddd
eeeeeeeeeeffffffffff
eeeeeeeeeeffffffffff
eeeeeeeeeeffffffffff`,
			}, {
				func() { l.Resize(2, 2) }, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
			},
		}
		testutil.TestComponent(t, l, w, tests)
	})

	t.Run("zero row", func(t *testing.T) {
		l := Grid([][]tui.Component{
			[]tui.Component{&TestComponent{Ch: 'a'}},
			nil,
			[]tui.Component{&TestComponent{Ch: 'b'}},
		})
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
