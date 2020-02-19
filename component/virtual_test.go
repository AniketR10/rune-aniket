package component

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/require"
)

func TestIntegrationScroll(t *testing.T) {
	width, height := 8, 4
	tabspaces := 4
	wrap := false
	virtualScroll := Virtual{C: newScroll(tabspaces, wrap, width, height)}
	virtualScroll.Resize(width, height)
	str := "AAAAAAAAAAAA\nBBBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDDDD"
	_, err := virtualScroll.C.(*Scroll).ReadFrom(strings.NewReader(str))
	require.NoError(t, err)

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
	v := Virtual{C: &TestComponent{Ch: '$'}}
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
