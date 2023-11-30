package vi

import (
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

// wraps text.CursorMouseDelegate to set vi states
type mouseDelegate struct {
	text.MouseDelegate
	vi *viHandlerImpl
}

func newMouseDelegate(vi *viHandlerImpl) text.MouseDelegate {
	return mouseDelegate{MouseDelegate: text.CursorMouseDelegate(&vi.cursor), vi: vi}
}

func (d mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	d.MouseDelegate.SetSelectionStart(pos)
	d.vi.setVisualMode()
}

func (d mouseDelegate) ClearSelection() {
	d.vi.setNormalMode()
	d.MouseDelegate.ClearSelection()
}
