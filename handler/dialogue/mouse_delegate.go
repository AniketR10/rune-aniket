package dialogue

import (
	"fmt"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/tcell/v3"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/clipboard"
)

func newMouseDelegate(
	l *component.ResponsiveList, clip clipboard.Register,
) text.MouseDelegate {
	return &mouseDelegate{list: l, clipboard: clip}
}

// satisfies text.MouseDelegate
type mouseDelegate struct {
	list          *component.ResponsiveList
	selection     component.WithAttributes
	selectionAttr term.Attributes
	clipboard     clipboard.Register
}

func (d *mouseDelegate) OnAction(ev term.Event, pos term.Coordinates, action text.MouseAction) bool {
	return false
}

func (d *mouseDelegate) ScrollUp(n int) (ok bool) {
	return d.list.SeekUp()
}

func (d *mouseDelegate) ScrollDown(n int) (ok bool) {
	return d.list.SeekDown()
}

func (d *mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	if d.selection != nil {
		d.ClearSelection()
	}
}

func (d *mouseDelegate) SetSelectionEnd(pos term.Coordinates) {
	node, ok := d.list.ElementAt(pos)
	if !ok {
		return
	}
	if d.selection != nil {
		d.ClearSelection()
	}
	content := node.Value().(*component.Span).Content().(*component.AttrSetter)
	d.selectionAttr = content.SetAttr(term.Attributes{Attrs: tcell.AttrReverse})
	d.selection = content

	err := d.clipboard.Copy(clipboard.DefaultRegisterID,
		clipboard.Data{Text: content.Content().(fmt.Stringer).String()})
	if err != nil {
		log.WithFields(log.Fields{
			logging.KeyClass: "dialogue.mouseDelegate",
			logging.KeyError: err,
		}).Error("copy to clipboard")
	}
}

func (d *mouseDelegate) ClearSelection() {
	if d.selection == nil {
		return
	}
	d.selection.SetAttr(d.selectionAttr)
	d.selection = nil
}

func (d *mouseDelegate) SelectWordAt(pos term.Coordinates) {
	d.SetSelectionEnd(pos)
}

func (d *mouseDelegate) SelectLine(y int) {
	pos := term.Coordinates{Y: y}
	d.SetSelectionEnd(pos)
}

func (d *mouseDelegate) Width() int {
	return d.list.SizeWidth()
}

func (d *mouseDelegate) Height() int {
	return d.list.SizeHeight()
}
