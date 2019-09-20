package fractal

import (
	"reflect"
	"github.com/nsf/termbox-go"
	"testing"
)

func TestFrameProxyMan(t *testing.T) {
	myManual := Manual{
		Summary: "sup",
		Keys: KeyMap{
			termbox.Event{Ch: 'j'}: {
				ID:          "wow",
				Description: "now",
			},
		},
	}
	handler := &TestHandler{Manual: myManual}
	if !reflect.DeepEqual(handler.Man(), NewFrameProxy(handler, 0, 0).Man()) {
		t.Errorf("did not proxy Man correctly")
	}
}

func TestFrameProxyCursor(t *testing.T) {
	handler := &TestHandler{}
	proxy := NewFrameProxy(handler, 0, 0)
	proxy.Frame.bwidth, proxy.Frame.bheight = 2, 2
	offsetCursor := handler.GetCursor()
	offsetCursor.X++
	offsetCursor.Y++

	if offsetCursor != proxy.GetCursor() {
		t.Errorf("did not proxy GetCursor correctly")
	}
}
