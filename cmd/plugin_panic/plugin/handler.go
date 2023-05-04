package plugin

import (
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
)

// Grantee returns this plugin's Grantee and the permissions required to run it.
func Grantee() (plugin.Grantee, []plugin.Permission) {
	return plugutil.NewCommandSplitHandler(plugutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationRight,
		Handler: func(grants []plugin.Grant, broker proto.MuxBroker,
			invokeWindow browserapi.Window, config config.Config) (browserapi.Handler, error) {
			return new(panicHandler), nil
		},
		Command: "panicPlugin",
	})
}

type panicHandler struct {
	comp tui.Component
}

func (h *panicHandler) Resize(width, height int) {
	if h.comp == nil {
		h.comp = component.NewStringWithConfig(`
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
`, component.StringConfig{
			Alignment: component.SpanAlignmentCentered,
		})
	}
	h.comp.Resize(width, height)
}

func (h *panicHandler) Draw(w term.Writer) {
	h.comp.Draw(w)
}

func (h *panicHandler) Handle(ev term.Event) (exit, handled bool) {
	panic("kaboom")
}

func (h *panicHandler) Cursor() (pos term.Coordinates, show bool) {
	return
}

func (h *panicHandler) Man() tui.Manual {
	return tui.Manual{}
}

func (h *panicHandler) Close() error {
	return nil
}
