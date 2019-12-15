package handler

import (
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/term"
)

func TestKeyMappedLessHandle(t *testing.T) {
	cases := getLessHandleTestFlow([19]term.Event{
		term.Event{},
		term.Event{Ch: 'k', Type: term.EventKey},
		term.Event{Ch: 'U', Type: term.EventKey},
		term.Event{Ch: '%', Type: term.EventKey},
		term.Event{Ch: 'l', Type: term.EventKey},
		term.Event{Key: term.KeyArrowRight, Type: term.EventKey},
		term.Event{Key: term.KeyArrowLeft, Type: term.EventKey},
		term.Event{Ch: 'G', Type: term.EventKey},
		term.Event{Ch: 'g', Type: term.EventKey},
		term.Event{Ch: '\\', Type: term.EventKey},
		term.Event{Ch: 'X', Type: term.EventKey},
		term.Event{Key: term.KeyBackspace, Type: term.EventKey},
		term.Event{Ch: 'X', Type: term.EventKey},
		term.Event{Ch: 'X', Type: term.EventKey},
		term.Event{Key: term.KeyEnter, Type: term.EventKey},
		term.Event{Ch: 'g', Type: term.EventKey},
		term.Event{Ch: 'N', Type: term.EventKey, Mod: term.ModAlt},
		term.Event{Ch: 'n', Type: term.EventKey, Mod: term.ModAlt},
		term.Event{},
	})
	less1, writer3 := setup(nil, 8, 4)
	testHandlerWorkflow(t, WithMapping(less1, map[term.Event]term.Event{
		term.Event{Ch: 'k', Type: term.EventKey}:                    term.Event{Ch: 'k', Type: term.EventKey},
		term.Event{Ch: 'U', Type: term.EventKey}:                    term.Event{Ch: 'j', Type: term.EventKey},
		term.Event{Ch: '%', Type: term.EventKey}:                    term.Event{Ch: 'h', Type: term.EventKey},
		term.Event{Ch: '\\', Type: term.EventKey}:                   term.Event{Ch: '/', Type: term.EventKey},
		term.Event{Key: term.KeyArrowRight, Type: term.EventKey}:    term.Event{Ch: '$', Type: term.EventKey},
		term.Event{Key: term.KeyArrowLeft, Type: term.EventKey}:     term.Event{Ch: '0', Type: term.EventKey},
		term.Event{Key: 'N', Type: term.EventKey, Mod: term.ModAlt}: term.Event{Ch: 'N', Type: term.EventKey},
		term.Event{Key: 'n', Type: term.EventKey, Mod: term.ModAlt}: term.Event{Ch: 'n', Type: term.EventKey},
	}), cases, writer3)
}

func TestKeyMappingMan(t *testing.T) {
	mySummary := "My Summary"
	myID := "myID"
	myDesc := "myDesc"
	kKey := term.Event{Ch: 'k', Type: term.EventKey}
	jKey := term.Event{Ch: 'j', Type: term.EventKey}

	var handler fractal.Handler
	handler = &TestHandler{Manual: fractal.Manual{
		Summary: mySummary,
		Keys: fractal.KeyMap{
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
	handler = WithMapping(handler, map[term.Event]term.Event{
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
	cursor, _ := handler.Cursor()
	kmCursor, _ := WithMapping(handler, nil).Cursor()
	if cursor != kmCursor {
		t.Errorf("did not proxy Cursor correctly")
	}
}
