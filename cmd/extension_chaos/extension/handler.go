package extension

import (
	"context"
	"fmt"
	"sync"
	"time"

	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationRight,
		Handler: func(ctx context.Context, cmd textapi.Command,
			grants []extension.Grant, broker proto.MuxBroker,
			invokeWindow browserapi.Window, config config.Config) (browserapi.Handler, error) {
			comp := component.NewStringWithConfig(copy, component.StringConfig{
				Alignment: component.SpanAlignmentCentered,
			})

			sleepDuration := 1 * time.Second
			panic := len(cmd.Args) == 0 || cmd.Args[0] == "panic"

			if len(cmd.Args) > 1 && cmd.Args[0] == "slow" {
				dur, err := time.ParseDuration(cmd.Args[1])
				if err != nil {
					return nil, fmt.Errorf("parse duration: %v", err)
				}
				sleepDuration = dur
			}
			h := &chaosHandler{sleepTime: sleepDuration, panic: panic, comp: comp}
			return browserapi.SyncHandler(new(sync.Mutex), h), nil
		},
		Command: textapi.CommandManual{
			Name: "chaosExtension",
			Summary: "This is extension is internal and for debugging purposes only. " +
				"If no argument is passed then 'panic' is assumed.",
			Synopsis: "[(panic|slow [duration])]",
		},
	})
}

const copy = `
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
