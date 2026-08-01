// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package appmenu

import (
	"reflect"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestKeyEquivalent(t *testing.T) {
	tests := []struct {
		name  string
		comb  term.KeyComb
		equiv string
		mods  uint
		ok    bool
	}{
		{
			name:  "meta character",
			comb:  term.KeyComb{Mod: term.ModMeta, Ch: 'q'},
			equiv: "q",
			mods:  modCommandMask,
			ok:    true,
		},
		{
			name:  "uppercase character implies shift",
			comb:  term.KeyComb{Mod: term.ModMeta, Ch: 'Q'},
			equiv: "q",
			mods:  modCommandMask | modShiftMask,
			ok:    true,
		},
		{
			name:  "ctrl meta character",
			comb:  term.KeyComb{Mod: term.ModCtrlMeta, Ch: 'f'},
			equiv: "f",
			mods:  modControlMask | modCommandMask,
			ok:    true,
		},
		{
			name:  "all modifiers",
			comb:  term.KeyComb{Mod: term.ModCtrlShiftMeta | term.ModAlt, Ch: 'a'},
			equiv: "a",
			mods:  modControlMask | modShiftMask | modOptionMask | modCommandMask,
			ok:    true,
		},
		{
			name:  "unmodified character",
			comb:  term.KeyComb{Ch: 'x'},
			equiv: "x",
			ok:    true,
		},
		{
			name:  "function key",
			comb:  term.KeyComb{Key: term.KeyF11},
			equiv: "\uF70E",
			ok:    true,
		},
		{
			name:  "arrow key with alt",
			comb:  term.KeyComb{Mod: term.ModAlt, Key: term.KeyArrowLeft},
			equiv: "\uF702",
			mods:  modOptionMask,
			ok:    true,
		},
		{
			name:  "enter",
			comb:  term.KeyComb{Mod: term.ModMeta, Key: term.KeyEnter},
			equiv: "\u000D",
			mods:  modCommandMask,
			ok:    true,
		},
		{
			name:  "space carries both key and ch",
			comb:  term.KeyComb{Mod: term.ModMeta, Key: term.KeySpace, Ch: ' '},
			equiv: " ",
			mods:  modCommandMask,
			ok:    true,
		},
		{
			name: "zero value has no equivalent",
			comb: term.KeyComb{},
		},
		{
			name: "mouse button has no equivalent",
			comb: term.KeyComb{Key: term.MouseLeft},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equiv, mods, ok := keyEquivalent(tt.comb)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if equiv != tt.equiv {
				t.Errorf("equiv = %q, want %q", equiv, tt.equiv)
			}
			if mods != tt.mods {
				t.Errorf("mods = %#x, want %#x", mods, tt.mods)
			}
		})
	}
}

func TestBuildSpec(t *testing.T) {
	quit := Command{Title: "Quit Rune", Command: "quit", Key: term.KeyComb{Mod: term.ModMeta, Ch: 'q'}}
	settings := Command{Title: "Settings...", Command: "config", Args: []string{"edit"}}
	fullscreen := Native{
		Title:    "Enter Full Screen",
		Selector: "toggleFullScreen:",
		Key:      term.KeyComb{Mod: term.ModCtrlMeta, Ch: 'f'},
	}

	specs, byTag := buildSpec([]Menu{
		{Title: "Rune", Items: []Item{settings, Separator{}, quit}},
		{Title: "View", Items: []Item{fullscreen}},
	})

	want := []menuSpec{
		{
			title: "Rune",
			items: []itemSpec{
				{title: "Settings...", tag: 1},
				{separator: true},
				{title: "Quit Rune", keyEquiv: "q", modifiers: modCommandMask, tag: 2},
			},
		},
		{
			title: "View",
			items: []itemSpec{
				{
					title:     "Enter Full Screen",
					selector:  "toggleFullScreen:",
					keyEquiv:  "f",
					modifiers: modControlMask | modCommandMask,
				},
			},
		},
	}
	if !reflect.DeepEqual(specs, want) {
		t.Errorf("specs = %+v, want %+v", specs, want)
	}

	wantTags := map[int]Command{1: settings, 2: quit}
	if !reflect.DeepEqual(byTag, wantTags) {
		t.Errorf("byTag = %+v, want %+v", byTag, wantTags)
	}
}

func TestActivateTag(t *testing.T) {
	quit := Command{Title: "Quit Rune", Command: "quit"}
	settings := Command{Title: "Settings...", Command: "config"}

	var got []Command
	Install([]Menu{
		{Title: "Rune", Items: []Item{
			settings,
			Separator{},
			Native{Title: "Hide", Selector: "hide:"},
			quit,
		}},
	}, func(c Command) { got = append(got, c) })

	activateTag(2)
	activateTag(1)
	// Tag 0 is AppKit's default for items without a command.
	activateTag(0)
	activateTag(99)

	want := []Command{quit, settings}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("activated = %+v, want %+v", got, want)
	}
}

func TestActivateTagWithoutInstall(t *testing.T) {
	Install(nil, nil)
	activateTag(1)
}
