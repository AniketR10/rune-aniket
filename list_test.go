package fractal

import (
	"testing"
)

func TestNewList(t *testing.T) {
	l := NewList(1)

	l.Resize(8, 4)

	if l.ElementHeight() != 1 || l.width != 8 || l.height != 4 {
		t.Errorf("sizes not initialized correctly: %+v", l)
	}

	var l2 List

	l2.Init(1)
	l.Resize(8, 4)

	if l.ElementHeight() != 1 || l.width != 8 || l.height != 4 {
		t.Errorf("sizes not initialized correctly: %+v", l)
	}
}

func TestListDraw(t *testing.T) {
	l := NewList(1)
	l2 := NewList(1)

	l.Resize(8, 4)

	l2.Resize(8, 4)

	w := newStringWriter(8, 4)

	tests := []testCase{
		{
			nil, `
        
        
        
        `,
		}, {
			func() { l.PushBack(&VirtualComponent{C: &TestComponent{Ch: 'X'}}) }, `
XXXXXXXX
        
        
        `,
		}, {
			func() { l.PushBack(&VirtualComponent{C: &TestComponent{Ch: 'Y'}}) }, `
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
			func() { l.PushFront(&VirtualComponent{C: &TestComponent{Ch: 'Z'}}) }, `
ZZZZZZZZ
XXXXXXXX
YYYYYYYY
        `,
		}, {
			func() {
				l2.PushFront(&VirtualComponent{C: &TestComponent{Ch: '$'}})
				l2.PushFront(&VirtualComponent{C: &TestComponent{Ch: '#'}})
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
			func() { l.Resize(4, 4) }, `
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
