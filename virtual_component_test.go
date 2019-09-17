package fractal

import "testing"

func TestIntegrationScroll(t *testing.T) {
	width, height := 8, 4
	tabspaces := 4
	wrap := false
	virtualScroll := VirtualComponent{C: newScroll(tabspaces, wrap, width, height)}
	virtualScroll.Resize(width, height)
	virtualScroll.C.(*Scroll).WriteStr("AAAAAAAAAAAA\nBBBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDDDD")

	w := newStringWriter(12, height)

	tests := []testCase{
		{
			nil, `
AAAAAAAA    
BBBBBBBB    
CCCCCCCC    
DDDDDDDD    `,
		},
		{
			func() { virtualScroll.Move(Coordinates{1, 1}) }, `
            
 AAAAAAAA   
 BBBBBBBB   
 CCCCCCCC   `,
		},
		{
			func() { virtualScroll.Move(Coordinates{3, 0}) }, `
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

	w := newStringWriter(width, height)

	tests := []testCase{
		{
			nil, `
$$$$
$$$$
$$$$
$$$$`,
		},
		{
			func() { v.Move(Coordinates{1, 1}) }, `
    
 $$$
 $$$
 $$$`,
		},
	}
	testWorkflow(t, &v, w, tests)
}
