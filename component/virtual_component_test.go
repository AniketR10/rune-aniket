package component

import (
	"testing"

	"github.com/ernestrc/fractal/term"
)

func TestIntegrationScroll(t *testing.T) {
	width, height := 8, 4
	tabspaces := 4
	wrap := false
	virtualScroll := VirtualComponent{C: newScroll(tabspaces, wrap, width, height)}
	virtualScroll.Resize(width, height)
	virtualScroll.C.(*Scroll).WriteString("AAAAAAAAAAAA\nBBBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDDDD")

	w := term.NewStringWriter(12, height)

	tests := []testCase{
		{
			nil, `
AAAAAAAA    
BBBBBBBB    
CCCCCCCC    
DDDDDDDD    `,
		},
		{
			func() { virtualScroll.Move(term.Coordinates{1, 1}) }, `
            
 AAAAAAAA   
 BBBBBBBB   
 CCCCCCCC   `,
		},
		{
			func() { virtualScroll.Move(term.Coordinates{3, 0}) }, `
   AAAAAAAA 
   BBBBBBBB 
   CCCCCCCC 
   DDDDDDDD `,
		},
	}

	testWorkflow(t, &virtualScroll, w, tests)
}

func TestVirtualDraw(t *testing.T) {
	width, height := 4, 4
	v := VirtualComponent{C: &TestComponent{Ch: '$'}}
	v.Resize(width, height)

	w := term.NewStringWriter(width, height)

	tests := []testCase{
		{
			nil, `
$$$$
$$$$
$$$$
$$$$`,
		},
		{
			func() { v.Move(term.Coordinates{1, 1}) }, `
    
 $$$
 $$$
 $$$`,
		},
	}
	testWorkflow(t, &v, w, tests)
}
