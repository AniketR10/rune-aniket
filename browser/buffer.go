package browser

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// This structure is just a wrap structure to be able to add certain
// properties to a tui.Handler.
type browserBuffer struct {
	filename string
	fileBuf  fileBuffer
	handler  tui.Handler
	free     bool
}

func (s *browserBuffer) Resize(width, height int) {
	s.handler.Resize(width, height)
}

func (s *browserBuffer) Draw(w term.Writer) {
	s.handler.Draw(w)
}

func (s *browserBuffer) Handle(ev term.Event) (bool, bool) {
	return s.handler.Handle(ev)
}

func (s *browserBuffer) Cursor() (pos term.Coordinates, show bool) {
	return s.handler.Cursor()
}

func (s *browserBuffer) Man() tui.Manual {
	return s.handler.Man()
}

func (s *browserBuffer) Close() error {
	if s.fileBuf != nil {
		err := s.fileBuf.Close()
		s.fileBuf = nil
		return err
	}
	return nil
}
