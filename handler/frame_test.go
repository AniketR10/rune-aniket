package handler

import (
	"reflect"
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/term"
)

func TestFrameProxyMan(t *testing.T) {
	myManual := fractal.Manual{
		Summary: "sup",
		Keys: fractal.KeyMap{
			term.Event{Ch: 'j'}: {
				ID:          "wow",
				Description: "now",
			},
		},
	}
	handler := &TestHandler{Manual: myManual}
	if !reflect.DeepEqual(handler.Man(), NewFrame(handler).Man()) {
		t.Errorf("did not proxy Man correctly")
	}
}

func TestFrameProxyCursor(t *testing.T) {
	handler := &TestHandler{}
	proxy := NewFrame(handler)
	proxy.Resize(4, 4)
	offsetCursor, _ := handler.Cursor()
	offsetCursor.X++
	offsetCursor.Y++

	if newCursor, _ := proxy.Cursor(); newCursor != offsetCursor {
		t.Errorf("did not proxy Cursor correctly")
	}
}
