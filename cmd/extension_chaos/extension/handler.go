package extension

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
)

const (
	commandChaosHandler              = "chaosHandler"
	commandChaosUpdateEventLatency   = "chaosUpdateEventLatency"
	commandChaosUpdateCommandLatency = "chaosUpdateCommandLatency"
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

func newChaosCommandHandler(
	ctx context.Context, ed textapi.Editor, grants []extension.Grant,
	broker proto.MuxBroker, pconfig config.Config,
) (extutil.CommandEventHandler, error) {
	ret := new(chaosCommandHandler)
	ret.ed = ed
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	ret.eventLatency.Store(time.Duration(0))
	ret.commandLatency.Store(time.Duration(0))

	var err error
	for _, grant := range grants {
		switch grant.Permission {
		case extension.PermissionBrowserWindowManager:
			ret.wm, err = browserextension.WindowManager(ctx, grant, broker)
			if err != nil {
				return nil, err
			}
		}
	}

	return ret, nil
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

func (s *chaosCommandHandler) log(level log.Level, msg string, args ...interface{}) {
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
	exit bool, err error,
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
		return false, h.handleNewChaosHandler(ctx, cmd)
	case commandChaosUpdateEventLatency:
		return false, h.handleUpdateEventLatency(ctx, cmd)
	case commandChaosUpdateCommandLatency:
		return false, h.handleUpdateCommandLatency(ctx, cmd)
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
		Alignment: component.SpanAlignmentCentered,
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

func (h *chaosHandler) Cursor() (
	pos term.Coordinates, style term.CursorStyle, show bool,
) {
	if !h.panic {
		time.Sleep(h.sleepTime)
	}
	return
}

func (h *chaosHandler) Man() tui.Manual {
	if !h.panic {
		time.Sleep(h.sleepTime)
	}
	return tui.Manual{}
}

func (h *chaosHandler) Close() error {
	if !h.panic {
		time.Sleep(h.sleepTime)
	}
	return nil
}
