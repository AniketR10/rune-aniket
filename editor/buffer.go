package editor

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/term"
)

// This structure is just a wrap structure to be able to add certain
// properties to a fractal.Handler editor.
type editorBuffer struct {
	filename string
	fileBuf  *FileBuffer
	editor   fractal.Handler
	node     *component.TileNode
}

func (s *editorBuffer) setNode(node *component.TileNode) (
	prev *component.TileNode,
) {
	prev = s.node
	s.node = node
	return
}

func (s *editorBuffer) Resize(width, height int) {
	s.editor.Resize(width, height)
}

func (s *editorBuffer) Draw(w fractal.Writer) {
	s.editor.Draw(w)
}

func (s *editorBuffer) Handle(ev term.Event) bool {
	return s.editor.Handle(ev)
}

func (s *editorBuffer) Cursor() (pos term.Coordinates, show bool) {
	return s.editor.Cursor()
}

func (s *editorBuffer) Man() fractal.Manual {
	return s.editor.Man()
}

func (s *editorBuffer) Close() error {
	if s.fileBuf != nil {
		err := s.fileBuf.Close()
		s.fileBuf = nil
		return err
	}
	return nil
}
