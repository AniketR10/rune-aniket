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

package lspcmd

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"unstable.build/go-tui/handler/locationpicker"
)

type locationEntry struct {
	uri     string
	rng     semanticapi.Range
	display string
}

var errNoLocations = errors.New("no location returned")

func navigateTo(
	rootURI workspaceapi.URI,
	e locationEntry, opener browserapi.ResourceOpener,
	wm browserapi.WindowManager,
	editor textapi.Editor, notify browserapi.Notifications,
	scheduleNextTick func(func()) bool,
) {
	scheduleNextTick(func() {
		if err := doNavigate(rootURI, e, opener, wm, editor); err != nil {
			_, _ = notify.Notify(browserapi.LevelError, "navigate: %s", err)
			return
		}
	})
}

func doNavigate(
	rootURI workspaceapi.URI,
	e locationEntry, opener browserapi.ResourceOpener,
	wm browserapi.WindowManager, editor textapi.Editor,
) error {
	uri, err := LspToURI(rootURI, e.uri)
	if err != nil {
		return err
	}
	h, err := opener.Open(uri)
	if err != nil {
		return err
	}
	w, err := wm.Focus()
	if err != nil {
		return err
	}
	if err := wm.SetWindowContent(w, h); err != nil && !errors.Is(err, browserapi.ErrTabNotFree) {
		return err
	}
	eh, err := editor.Editor(uri)
	if err != nil {
		return err
	}
	target := PosToCoord(e.rng.Start)
	if err := editor.SetCursor(eh, target); err != nil {
		if cur, curErr := editor.Cursor(eh); curErr == nil && cur == target {
			return nil
		}
		return err
	}
	return nil
}

func locationsFromResult(r semanticapi.LocationResult) []locationEntry {
	var entries []locationEntry
	if r.Location != nil {
		entries = append(entries, locationFromLoc(*r.Location))
	}
	for _, loc := range r.Locations {
		entries = append(entries, locationFromLoc(loc))
	}
	for _, ll := range r.LocationLinks {
		entries = append(entries, locationEntry{
			uri: ll.TargetURI,
			rng: ll.TargetSelectionRange,
			display: fmt.Sprintf("%s:%d",
				trimFilePrefix(ll.TargetURI), ll.TargetSelectionRange.Start.Line+1),
		})
	}
	return entries
}

func pickerEntries(rootURI workspaceapi.URI, entries []locationEntry) []locationpicker.Entry {
	ret := make([]locationpicker.Entry, len(entries))
	for i, e := range entries {
		uri, err := LspToURI(rootURI, e.uri)
		if err != nil {
			continue
		}
		ret[i] = locationpicker.Entry{
			URI:     uri,
			Range:   e.rng,
			Display: e.display,
		}
	}
	return ret
}

func locationFromLoc(loc semanticapi.Location) locationEntry {
	return locationEntry{
		uri:     loc.URI,
		rng:     loc.Range,
		display: fmt.Sprintf("%s:%d", trimFilePrefix(loc.URI), loc.Range.Start.Line+1),
	}
}

// presentLocations navigates directly when a single entry remains and
// otherwise floats a location picker. It may be called from any goroutine:
// all UI work is scheduled onto the event loop.
func presentLocations(
	name string, rootURI workspaceapi.URI, entries []locationEntry,
	opener browserapi.ResourceOpener, wm browserapi.WindowManager,
	editor textapi.Editor, notify browserapi.Notifications,
	fs workspaceapi.FileSystem, scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, cfg locationpicker.Config, log *slog.Logger,
) {
	if len(entries) == 1 {
		navigateTo(rootURI, entries[0], opener, wm, editor, notify, scheduleNextTick)
		return
	}
	scheduleNextTick(func() {
		picker := locationpicker.New(
			pickerEntries(rootURI, entries), wm, fs, scheduleNextTick, parser, cfg, log,
		)
		picker.SetOnSelect(func(idx int) {
			navigateTo(rootURI, entries[idx], opener, wm, editor, notify, scheduleNextTick)
		})
		win, err := wm.Floating(picker, browserapi.FloatingConfig{
			Alignment: component.AlignmentCentered,
		})
		if err != nil {
			_, _ = notify.Notify(browserapi.LevelError, "%s: %s", name, err)
			return
		}
		picker.SetWindow(win)
	})
}

func trimFilePrefix(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

func enrichEntries(
	entries []locationEntry, rootURI workspaceapi.URI,
) []locationEntry {
	for i, e := range entries {
		uri, err := LspToURI(rootURI, e.uri)
		var rel string
		if err == nil {
			rel = workspaceapi.RelPath(rootURI, uri)
		} else {
			rel = trimFilePrefix(e.uri)
		}
		entries[i].display = fmt.Sprintf("%s:%d", rel, e.rng.Start.Line+1)
	}
	return entries
}
