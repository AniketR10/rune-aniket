// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package texttest

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/text"
)

func TestIconsBarSetLocationListDrivesWidth(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString("a\nb\nc")
	scroll := component.NewScroll(buf)
	h := &interceptOnlyLocationListsHandler{testHandler: newTestHandler(scroll)}

	bar := text.WithIconsBar(vctrl.NopService(), false, h, buf, scroll, text.IconsBarConfig{
		ScheduleNextTick: func(fn func()) bool {
			fn()
			return true
		},
	})
	bar.Resize(7, 3)
	w := term.NewStringWriter(7, 3)

	comptest.TestComponent(t, bar, w, []comptest.TestCase{{Expected: `
a      
b      
c      `}})

	bar.SetLocationList(textapi.LocationPriorityInfo, "marks", text.LocationSlice([]textapi.Location{{
		From: term.Coordinates{Y: 1},
		To:   term.Coordinates{Y: 2},
		Icon: "!",
	}}))
	comptest.TestComponent(t, bar, w, []comptest.TestCase{{Expected: `
  a    
! b    
  c    `}})

	bar.SetLocationList(textapi.LocationPriorityWarning, "breakpoints", text.LocationSlice([]textapi.Location{{
		From: term.Coordinates{Y: 1},
		To:   term.Coordinates{Y: 2},
		Icon: "+",
	}}))
	comptest.TestComponent(t, bar, w, []comptest.TestCase{{Expected: `
  a    
+ b    
  c    `}})

	bar.SetLocationList(textapi.LocationPriorityInfo, "marks", nil)
	bar.SetLocationList(textapi.LocationPriorityWarning, "breakpoints", nil)
	comptest.TestComponent(t, bar, w, []comptest.TestCase{{Expected: `
a      
b      
c      `}})
}

func TestIconsBarUsesIconWidthAndForegroundAttr(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString("a\nb")
	scroll := component.NewScroll(buf)
	h := &interceptOnlyLocationListsHandler{testHandler: newTestHandler(scroll)}

	bar := text.WithIconsBar(vctrl.NopService(), false, h, buf, scroll, text.IconsBarConfig{
		ScheduleNextTick: func(fn func()) bool {
			fn()
			return true
		},
	})
	bar.Resize(7, 2)
	bar.SetLocationList(textapi.LocationPriorityInfo, "diagnostics", text.LocationSlice([]textapi.Location{{
		From: term.Coordinates{Y: 1},
		To:   term.Coordinates{Y: 2},
		Icon: "",
		Attr: term.Attributes{Bg: tcell.ColorRed},
	}}))

	w := term.NewStringWriter(7, 2)
	comptest.TestComponent(t, bar, w, []comptest.TestCase{{Expected: `
   a   
  b   `}})

	w = term.NewStringWriter(7, 2)
	w.ForegroundCh = 'F'
	w.BackgroundCh = 'B'
	bar.Draw(w)
	require.NoError(t, w.Flush())
	assert.Equal(t, "   a   \nF  b   ", w.String())
	iconCell := w.Cells()[7]
	assert.Equal(t, tcell.ColorRed, iconCell.Fg)
	assert.Equal(t, tcell.ColorDefault, iconCell.Bg)
}

func TestIconsBarDrawsAllIconsWhenWrappedHandlerConsumesList(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString("a\nb\nc")
	scroll := component.NewScroll(buf)
	h := &consumeLocationListsHandler{testHandler: newTestHandler(scroll)}

	bar := text.WithIconsBar(vctrl.NopService(), false, h, buf, scroll, text.IconsBarConfig{
		ScheduleNextTick: func(fn func()) bool {
			fn()
			return true
		},
	})
	bar.Resize(7, 3)
	bar.SetLocationList(textapi.LocationPriorityInfo, "diagnostics", text.LocationSlice([]textapi.Location{
		{From: term.Coordinates{Y: 0}, To: term.Coordinates{Y: 1}, Icon: "1"},
		{From: term.Coordinates{Y: 1}, To: term.Coordinates{Y: 2}, Icon: "2"},
		{From: term.Coordinates{Y: 2}, To: term.Coordinates{Y: 3}, Icon: "3"},
	}))

	w := term.NewStringWriter(7, 3)
	comptest.TestComponent(t, bar, w, []comptest.TestCase{{Expected: `
1 a    
2 b    
3 c    `}})
	assert.Equal(t, 3, h.consumed)
}

func TestIconsBarRendersOneIconForMultilineLocation(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString("a\nb\nc\nd")
	scroll := component.NewScroll(buf)
	h := &interceptOnlyLocationListsHandler{testHandler: newTestHandler(scroll)}

	bar := text.WithIconsBar(vctrl.NopService(), false, h, buf, scroll, text.IconsBarConfig{
		ScheduleNextTick: func(fn func()) bool {
			fn()
			return true
		},
	})
	bar.Resize(7, 4)
	bar.SetLocationList(textapi.LocationPriorityInfo, "diagnostics", text.LocationSlice([]textapi.Location{{
		From: term.Coordinates{Y: 0},
		To:   term.Coordinates{Y: 3},
		Icon: "!",
	}}))

	w := term.NewStringWriter(7, 4)
	comptest.TestComponent(t, bar, w, []comptest.TestCase{{Expected: `
! a    
  b    
  c    
  d    `}})
}

func TestIconsBarNormalizesReversedLocationCoordinates(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString("a\nb\nc\nd")
	scroll := component.NewScroll(buf)
	h := &interceptOnlyLocationListsHandler{testHandler: newTestHandler(scroll)}

	bar := text.WithIconsBar(vctrl.NopService(), false, h, buf, scroll, text.IconsBarConfig{
		ScheduleNextTick: func(fn func()) bool {
			fn()
			return true
		},
	})
	bar.Resize(7, 4)
	bar.SetLocationList(textapi.LocationPriorityInfo, "diagnostics", text.LocationSlice([]textapi.Location{{
		From: term.Coordinates{Y: 3},
		To:   term.Coordinates{Y: 1},
		Icon: "!",
	}}))

	w := term.NewStringWriter(7, 4)
	comptest.TestComponent(t, bar, w, []comptest.TestCase{{Expected: `
  a    
! b    
  c    
  d    `}})
}

func TestGitBarDraw(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString(copy)
	fs := &testFoldsService{}
	fs.view = buf.WithView(fs)
	scroll := component.NewScroll(buf)
	h := newTestHandler(scroll)
	var wg sync.WaitGroup
	var mu sync.Mutex
	cb := func(fn func()) bool {
		mu.Lock()
		defer mu.Unlock()
		fn()
		wg.Done()
		return true
	}
	uri, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)

	registry := text.NewFileCommandRegistry(uri, newWorkspaceRegistry())
	ed := &TestEditor{}
	mockSvc := &differ{}
	wg.Add(1)
	mu.Lock()
	cfg := text.IconsBarConfig{CommandRegistry: registry, ScheduleNextTick: cb, Publisher: ed}
	bar := text.WithGitBar(mockSvc, h, buf, scroll, cfg)
	bar.Resize(15, 10)
	mu.Unlock()
	w := term.NewStringWriter(15, 10)

	tests := []comptest.TestCase{
		{Expected: `
  package main 
               
  import (     
      "fmt"    
+              
      "github.c
  )            
               
  func main() {
-     fmt.Print`,
		},
	}
	wg.Wait()
	mu.Lock()
	comptest.TestComponent(t, bar, w, tests)
	require.True(t, scroll.SeekDown())
	mu.Unlock()

	tests = []comptest.TestCase{
		{Expected: `
               
  import (     
      "fmt"    
+              
      "github.c
  )            
               
  func main() {
-     fmt.Print
      for i := `,
		},
	}
	mu.Lock()
	comptest.TestComponent(t, bar, w, tests)
	mu.Unlock()

	require.NoError(t, bar.Close())

	// unsubscribes
	require.Equal(t, 2, len(ed.subs))
	assert.Equal(t, 0, len(ed.subs[textapi.EventTypeFlush]))
	assert.Equal(t, 0, len(ed.subs[textapi.EventTypeFocus]))
}

type interceptOnlyLocationListsHandler struct {
	*testHandler
}

func (h *interceptOnlyLocationListsHandler) SetLocationList(
	pri textapi.LocationPriority, ID string, loc text.LocationList,
) {
	h.testHandler.SetLocationList(pri, ID, loc)
}

func (*interceptOnlyLocationListsHandler) LocationLists() []text.LocationSet {
	panic("icons bar should not read back LocationLists")
}

type consumeLocationListsHandler struct {
	*testHandler
	consumed int
}

func (h *consumeLocationListsHandler) SetLocationList(
	pri textapi.LocationPriority, ID string, loc text.LocationList,
) {
	h.consumed = 0
	for curr, ok := loc.Current(); ok; curr, ok = loc.Next() {
		if curr.Icon != "" {
			h.consumed++
		}
	}
	h.testHandler.SetLocationList(pri, ID, loc)
}

func (*consumeLocationListsHandler) LocationLists() []text.LocationSet {
	panic("icons bar should not read back LocationLists")
}

type noDiffDiffer struct{}

func (noDiffDiffer) ListRemotes(context.Context, workspaceapi.URI) ([]string, error) {
	return nil, nil
}

func (noDiffDiffer) ShortRef(context.Context, workspaceapi.URI) (string, error) {
	return "main", nil
}

func (noDiffDiffer) Diff(context.Context, workspaceapi.URI) (vctrl.FileDiff, error) {
	return vctrl.FileDiff{}, nil
}

func (noDiffDiffer) CurrentCommit(context.Context, workspaceapi.URI) (string, error) {
	panic("unimplemented")
}

func (noDiffDiffer) RemoteURL(context.Context, workspaceapi.URI, string) (string, error) {
	panic("unimplemented")
}

func (noDiffDiffer) RelPath(context.Context, string) (string, error) {
	panic("unimplemented")
}
