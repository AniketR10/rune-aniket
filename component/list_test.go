package component

import (
	"testing"

	"github.com/ernestrc/fractal"
)

func TestNewList(t *testing.T) {
	l := NewList(1, 8, 4, 1, 1)

	if l.ElementHeight() != 1 || l.Width() != 8 || l.Height() != 4 {
		t.Errorf("sizes not initialized correctly: %+v", l)
	}

	if x, y := l.Position(); x != 1 || y != 1 {
		t.Errorf("position not initialized correctly: %+v", l)
	}

	var l2 List

	l2.Init(1, 8, 4, 1, 1)

	if l.ElementHeight() != 1 || l.Width() != 8 || l.Height() != 4 {
		t.Errorf("sizes not initialized correctly: %+v", l)
	}

	if x, y := l.Position(); x != 1 || y != 1 {
		t.Errorf("position not initialized correctly: %+v", l)
	}
}

func TestListDraw(t *testing.T) {
	l := NewList(1, 8, 4, 0, 0)
	l2 := NewList(1, 8, 4, 0, 0)

	w := fractal.String(8, 4)

	tests := []testCase{
		{
			nil, `
        
        
        
        `,
		}, {
			func() { l.PushBack(&Fill{Ch: 'X'}) }, `
XXXXXXXX
        
        
        `,
		}, {
			func() { l.PushBack(&Fill{Ch: 'Y'}) }, `
XXXXXXXX
YYYYYYYY
        
        `,
		}, {
			l.SeekDown, `
XXXXXXXX
YYYYYYYY
        
        `,
		}, {
			l.SeekUp, `
XXXXXXXX
YYYYYYYY
        
        `,
		}, {
			func() { l.PushFront(&Fill{Ch: 'Z'}) }, `
ZZZZZZZZ
XXXXXXXX
YYYYYYYY
        `,
		}, {
			func() {
				l2.PushFront(&Fill{Ch: '$'})
				l2.PushFront(&Fill{Ch: '#'})
				l.PushBackList(l2)
			}, `
ZZZZZZZZ
XXXXXXXX
YYYYYYYY
########`,
		}, {
			l.SeekUp, `
ZZZZZZZZ
XXXXXXXX
YYYYYYYY
########`,
		}, {
			l.SeekDown, `
XXXXXXXX
YYYYYYYY
########
$$$$$$$$`,
		}, {
			l.SeekDown, `
XXXXXXXX
YYYYYYYY
########
$$$$$$$$`,
		}, {
			func() { l.Move(1, 1) }, `
        
 XXXXXXX
 YYYYYYY
 #######`,
		}, {
			func() { l.Resize(4, 4) }, `
        
 XXXX   
 YYYY   
 ####   `,
		}, {
			func() { l.Move(0, 0) }, `
XXXX    
YYYY    
####    
$$$$    `,
		}, {
			func() { l.SetElementHeight(2) }, `
XXXX    
XXXX    
YYYY    
YYYY    `,
		}, {
			func() { l.Resize(8, 4) }, `
XXXXXXXX
XXXXXXXX
YYYYYYYY
YYYYYYYY`,
		}, {
			func() { l.SetElementHeight(3) }, `
XXXXXXXX
XXXXXXXX
XXXXXXXX
        `,
		}, {
			l.SeekDown, `
YYYYYYYY
YYYYYYYY
YYYYYYYY
        `,
		}, {
			l.SeekEnd, `
$$$$$$$$
$$$$$$$$
$$$$$$$$
        `,
		}, {
			l.SeekStart, `
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ
        `,
		},
	}

	testWorkflow(t, l, w, tests)
}
