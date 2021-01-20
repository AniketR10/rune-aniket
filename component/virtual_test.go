package component

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
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

	tests := []testutil.ComponentTestCase{
		{
			nil, `
AAAAAAAA    
BBBBBBBB    
CCCCCCCC    
DDDDDDDD    `,
		},
		{
			func() {
				virtualScroll.Resize(8, 3)
				virtualScroll.Move(term.Coordinates{1, 1})
			}, `
            
 AAAAAAAA   
 BBBBBBBB   
 CCCCCCCC   `,
		},
		{
			func() {
				virtualScroll.Resize(8, 4)
				virtualScroll.Move(term.Coordinates{3, 0})
			}, `
   AAAAAAAA 
   BBBBBBBB 
   CCCCCCCC 
   DDDDDDDD `,
		},
	}

	testutil.TestComponent(t, &virtualScroll, w, tests)
}

func TestVirtualDraw(t *testing.T) {
	width, height := 4, 4
	v := Virtual{C: &TestComponent{Ch: '$'}}
	v.Resize(width, height)

	w := term.NewStringWriter(width, height)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
$$$$
$$$$
$$$$
$$$$`,
		},
		{
			func() {
				v.Resize(3, 3)
				v.Move(term.Coordinates{1, 1})
			}, `
    
 $$$
 $$$
 $$$`,
		},
	}
	testutil.TestComponent(t, &v, w, tests)
}
