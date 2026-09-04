// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package extension

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

const (
	commandChaosHandler              = "chaoshandler"
	commandChaosUpdateEventLatency   = "chaoseventlatency"
	commandChaosUpdateCommandLatency = "chaoscommandlatency"
	copy                             = `
              . . .                         
              \|/                          
            '--+--'                        
              /|\                          
             ' | '                         
               |                           
               |                           
           ,--'#'--.                       
           |#######|                       
        _.-'#######'-._                    
     ,-'###############'-.                 
   ,'#####################',               
  /#########################\              
 |###########################|             
|#############################|            
|#############################|            
|#############################|            
|#############################|            
 |###########################|             
  \#########################/              
   '.#####################,'               
     '._###############_,'                 
        '--..#####..--'
`
)

type chaosCommandHandler struct {
	ctx            context.Context
	cancelCtx      func()
	ed             textapi.Editor
	wm             browserapi.WindowManager
	eventLatency   atomic.Value
	commandLatency atomic.Value
}

// NewExtension returns the chaos extension and its metadata.
func NewExtension() (extensionapi.WorkspaceExtension, extensionapi.Metadata) {
	return workspaceExtension{}, extensionapi.Metadata{
		DeveloperID:      "Unstable Build",
		DeveloperEmail:   "it@unstable.build",
		DeveloperKey:     "064D4ABCFA6D9338",
		ExtensionID:      "chaos",
		ExtensionName:    "Chaos Extension",
		ExtensionVersion: "development",
		Permissions: extensionapi.NewPermissions(
			extensionapi.PermissionBrowserWindowManager,
			extensionapi.PermissionEditor,
			extensionapi.PermissionCommands,
		),
	}
}

type workspaceExtension struct{}

func (workspaceExtension) ExtendWorkspace(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	h := newChaosCommandHandler(ctx, w.Editor(ctx), w.WindowManager(ctx), cfg)
	for _, cmd := range ChaosHandlerCommands {
		if err := w.RegisterCommand(cmd, h); err != nil {
			return fmt.Errorf("register command %q: %w", cmd.Name, err)
		}
	}
	if err := h.ed.SubscribeEvents(ChaosHandlerEvents, h); err != nil {
		return fmt.Errorf("subscribe events: %w", err)
	}
	return nil
}

func newChaosCommandHandler(
	ctx context.Context, ed textapi.Editor, wm browserapi.WindowManager, pconfig config.Config,
) *chaosCommandHandler {
	ret := new(chaosCommandHandler)
	ret.ed = ed
	ret.wm = wm
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	ret.eventLatency.Store(time.Duration(0))
	ret.commandLatency.Store(time.Duration(0))
	return ret
}

func (h *chaosCommandHandler) Handle(ctx context.Context, ev textapi.Event) bool {
	h.log(log.TraceLevel, "handle event start")
	timer := time.NewTimer(h.eventLatency.Load().(time.Duration))
	select {
	case <-h.ctx.Done():
	case <-timer.C:
	}
	h.log(log.TraceLevel, "handle event end")
	return false
}

func (s *chaosCommandHandler) log(level log.Level, msg string, args ...any) {
	log.WithField(logging.KeyClass, "extchaos.chaosCommandHandler").
		Logf(level, msg, args...)
}

func (t *chaosCommandHandler) Complete(
	ctx context.Context, name string, args []string,
) (iterator.Iterator[string], error) {
	if name != commandChaosHandler || len(args) > 1 {
		return iterator.FromSlice[string](nil), nil
	}

	return iterator.FromSlice[string]([]string{"panic", "slow"}), nil
}

func (h *chaosCommandHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
	err error,
) {
	h.log(log.TraceLevel, "handle command start")
	defer h.log(log.TraceLevel, "handle command end")

	timer := time.NewTimer(h.commandLatency.Load().(time.Duration))
	select {
	case <-h.ctx.Done():
		err = h.ctx.Err()
		return
	case <-timer.C:
	}
	switch cmd.Name {
	case commandChaosHandler:
		return h.handleNewChaosHandler(ctx, cmd)
	case commandChaosUpdateEventLatency:
		return h.handleUpdateEventLatency(ctx, cmd)
	case commandChaosUpdateCommandLatency:
		return h.handleUpdateCommandLatency(ctx, cmd)
	}

	return
}

func (h *chaosCommandHandler) Close() error {
	h.cancelCtx()
	return nil
}

func (h *chaosCommandHandler) handleUpdateEventLatency(
	ctx context.Context, cmd textapi.Command,
) error {
	return h.handleUpdateDuration(ctx, cmd, &h.eventLatency)
}

func (h *chaosCommandHandler) handleUpdateCommandLatency(
	ctx context.Context, cmd textapi.Command,
) error {
	return h.handleUpdateDuration(ctx, cmd, &h.commandLatency)
}

func (h *chaosCommandHandler) handleUpdateDuration(
	ctx context.Context, cmd textapi.Command,
	duration *atomic.Value,
) error {
	var nextDuration time.Duration
	if len(cmd.Args) > 0 {
		dur, err := time.ParseDuration(cmd.Args[0])
		if err != nil {
			return fmt.Errorf("parse duration: %w", err)
		}
		nextDuration = dur
	}

	duration.Store(nextDuration)
	return nil
}

func (h *chaosCommandHandler) handleNewChaosHandler(
	ctx context.Context, cmd textapi.Command,
) error {
	comp := component.NewStringWithConfig(copy, component.StringConfig{
		Alignment: component.AlignmentCentered,
	})

	panic := len(cmd.Args) == 0 || cmd.Args[0] == "panic"

	var sleepDuration time.Duration
	if len(cmd.Args) > 1 && cmd.Args[0] == "slow" {
		dur, err := time.ParseDuration(cmd.Args[1])
		if err != nil {
			return fmt.Errorf("parse duration: %w", err)
		}
		sleepDuration = dur
	}

	tuiHandler := &chaosHandler{sleepTime: sleepDuration, panic: panic, comp: comp}
	syncHandler := browserapi.SyncHandler(new(sync.Mutex), tuiHandler)

	_, err := h.wm.Split(browserapi.OrientationDefault, cmd.Window, syncHandler)
	if err != nil {
		return fmt.Errorf("split: %w", err)
	}
	return nil
}

type chaosHandler struct {
	comp      tui.Component
	panic     bool
	sleepTime time.Duration
}

func (h *chaosHandler) Resize(width, height int) {
	if !h.panic {
		time.Sleep(h.sleepTime)
	}
	h.comp.Resize(width, height)
}

func (h *chaosHandler) Draw(w term.Writer) {
	if !h.panic {
		time.Sleep(h.sleepTime)
	}
	h.comp.Draw(w)
}

func (h *chaosHandler) Handle(ev term.Event) (exit, handled bool) {
	if h.panic {
		panic("kaboom")
	}
	time.Sleep(h.sleepTime)
	handled = true
	return
}

func (h *chaosHandler) Selection() (string, bool) {
	if !h.panic {
		time.Sleep(h.sleepTime)
	}
	return "", false
}

func (h *chaosHandler) Cursor() (
	pos term.Coordinates, style term.CursorStyle, show bool,
) {
	if !h.panic {
		time.Sleep(h.sleepTime)
	}
	return
}

func (h *chaosHandler) Close() error {
	if !h.panic {
		time.Sleep(h.sleepTime)
	}
	return nil
}
