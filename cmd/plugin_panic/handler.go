package main

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

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
