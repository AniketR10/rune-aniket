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

package vte

import (
	"context"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// DefaultConfig returns a sane default Config.
func DefaultConfig() Config {
	return Config{
		Clipboard:                clipboard.NewInMemory(),
		ClipboardRegister:        clipboard.DefaultRegisterID,
		ScheduleNextTick:         func(cb func()) bool { cb(); return true },
		RingBell:                 func() {},
		SelectionAttributes:      term.Attributes{Attrs: term.AttrReverse},
		NeedsAttentionAttributes: term.Attributes{Attrs: term.AttrBlink},
		DynamicTabName:           false,
		MaxLines:                 10_000,
		MinWidth:                 0,
	}
}

// Config configures Handler.
type Config struct {
	// CommandAndArgs is the program to run. Otherwise whatever is set
	// on the $SHELL environment variable is used.
	CommandAndArgs    []string
	Clipboard         clipboard.Register
	ClipboardRegister string

	// ScheduleNextTick schedules an arbitrary function to be run
	// in the next event-loop tick.
	ScheduleNextTick func(func()) bool

	// Use the VTE's title as the tab name.
	DynamicTabName bool

	// RingBell writes to the raw pty directly, bypassing the event loop.
	// This should only be used when called from the an event loop goroutine.
	RingBell func()
	Watcher  workspaceapi.ProcessWatcher

	Attributes               term.Attributes
	SelectionAttributes      term.Attributes
	NeedsAttentionAttributes term.Attributes
	MaxLines                 int
	// Bell overrides the default bell trigger. This is useful for non-standard
	// shells like the fish shell, which don't trigger the bell with the standard
	// escape sequence.
	Bell []byte

	// MinWidth helps optimize growing and shrinking rows upon resize.
	MinWidth int

	// Modal enables entering modal mode via Esc key.
	// Changing mode to 'INSERT' mode switches back to
	// the shell being in control of the input.
	Modal bool

	// WidthHint and HeightHint hint allows emulator.Handler to better configure the
	// initial buffer size.
	WidthHint  int
	HeightHint int

	Debug bool

	// CommandExpander, if non-nil, is invoked in a background
	// goroutine after the pty is created but before the foreign
	// command is started. ExpandCommand receives the joined
	// command line (CommandAndArgs joined with " ") and returns
	// the resolved line to actually execute. The pty exists while
	// the expander runs, so the host UI is interactive (resize,
	// draw) but no foreign process is attached yet.
	//
	// Returning an error aborts the spawn; the error is written to
	// the pty slave so it surfaces in the floating window like any
	// other failure, and the Watcher fires with that error.
	CommandExpander CommandExpander

	// SpawnTimeout bounds the synchronous spawn RPCs issued during
	// initialization (Terminal.NewPty and Executor.StartCommand).
	// On a remote workspace whose transport has stalled, those RPCs
	// can otherwise block forever and wedge whoever is constructing
	// the vte (e.g. the host event loop). Zero disables the bound.
	// It does not limit the lifetime of the spawned command.
	SpawnTimeout time.Duration
}

// CommandExpander resolves a vte command line before the foreign
// command is started. Implementations may run arbitrary I/O (e.g.
// resolve $(...) via the workspace executor); they are invoked off
// the event-loop goroutine so they may block.
type CommandExpander interface {
	ExpandCommand(ctx context.Context, line string) (string, error)
}

func (c Config) scheduleBell() {
	c.ScheduleNextTick(c.RingBell)
}
