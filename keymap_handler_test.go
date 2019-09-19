package fractal

import (
	"termbox"
	"testing"
)

func TestKeyMappedLessHandle(t *testing.T) {
	cases := getLessHandleTestFlow([19]termbox.Event{
		termbox.Event{},
		termbox.Event{Ch: 'k', Type: termbox.EventKey},
		termbox.Event{Ch: 'U', Type: termbox.EventKey},
		termbox.Event{Ch: '%', Type: termbox.EventKey},
		termbox.Event{Ch: 'l', Type: termbox.EventKey},
		termbox.Event{Key: termbox.KeyArrowRight, Type: termbox.EventKey},
		termbox.Event{Key: termbox.KeyArrowLeft, Type: termbox.EventKey},
		termbox.Event{Ch: 'G', Type: termbox.EventKey},
		termbox.Event{Ch: 'g', Type: termbox.EventKey},
		termbox.Event{Ch: '\\', Type: termbox.EventKey},
		termbox.Event{Ch: 'X', Type: termbox.EventKey},
		termbox.Event{Key: termbox.KeyBackspace, Type: termbox.EventKey},
		termbox.Event{Ch: 'X', Type: termbox.EventKey},
		termbox.Event{Ch: 'X', Type: termbox.EventKey},
		termbox.Event{Key: termbox.KeyEnter, Type: termbox.EventKey},
		termbox.Event{Ch: 'g', Type: termbox.EventKey},
		termbox.Event{Ch: 'N', Type: termbox.EventKey, Mod: termbox.ModAlt},
		termbox.Event{Ch: 'n', Type: termbox.EventKey, Mod: termbox.ModAlt},
		termbox.Event{},
	})
	less1, writer3 := setup(nil, 8, 4)
	testHandlerWorkflow(t, WithMapping(less1, map[termbox.Event]termbox.Event{
		termbox.Event{Ch: 'k', Type: termbox.EventKey}:                       termbox.Event{Ch: 'k', Type: termbox.EventKey},
		termbox.Event{Ch: 'U', Type: termbox.EventKey}:                       termbox.Event{Ch: 'j', Type: termbox.EventKey},
		termbox.Event{Ch: '%', Type: termbox.EventKey}:                       termbox.Event{Ch: 'h', Type: termbox.EventKey},
		termbox.Event{Ch: '\\', Type: termbox.EventKey}:                      termbox.Event{Ch: '/', Type: termbox.EventKey},
		termbox.Event{Key: termbox.KeyArrowRight, Type: termbox.EventKey}:    termbox.Event{Ch: '$', Type: termbox.EventKey},
		termbox.Event{Key: termbox.KeyArrowLeft, Type: termbox.EventKey}:     termbox.Event{Ch: '0', Type: termbox.EventKey},
		termbox.Event{Key: 'N', Type: termbox.EventKey, Mod: termbox.ModAlt}: termbox.Event{Ch: 'N', Type: termbox.EventKey},
		termbox.Event{Key: 'n', Type: termbox.EventKey, Mod: termbox.ModAlt}: termbox.Event{Ch: 'n', Type: termbox.EventKey},
	}), cases, writer3)
}

func TestKeyMappingMan(t *testing.T) {
	mySummary := "My Summary"
	myID := "myID"
	myDesc := "myDesc"
	kKey := termbox.Event{Ch: 'k', Type: termbox.EventKey}
	jKey := termbox.Event{Ch: 'j', Type: termbox.EventKey}

	var handler Handler
	handler = &TestHandler{Manual: Manual{
		Summary: mySummary,
		Keys: KeyMap{
			kKey: {
				ID:          myID,
				Description: myDesc,
			},
			jKey: {
				ID:          "",
				Description: "",
			},
		},
	}}

	manualBefore := handler.Man()
	handler = WithMapping(handler, map[termbox.Event]termbox.Event{
		kKey: jKey,
	})

	manualAfter := handler.Man()

	if manualBefore.Keys[kKey] != manualAfter.Keys[jKey] ||
		len(manualBefore.Keys) != len(manualAfter.Keys) ||
		len(manualAfter.Keys) != 2 {
		t.Errorf("Manual mapping not correct")
	}
}

func TestKeyMappingCursor(t *testing.T) {
	handler := &TestHandler{}
	if handler.GetCursor() != WithMapping(handler, nil).GetCursor() {
		t.Errorf("did not proxy GetCursor correctly")
	}
}
