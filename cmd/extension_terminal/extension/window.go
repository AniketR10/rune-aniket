package extension

import (
	browserapi "unstable.build/go-tui/api/browser"
	termutil "unstable.build/go-tui/cmd/extension_terminal/util"
	"unstable.build/go-tui/component/notifications"
)

var _ termutil.WindowManipulator = (*windowManipulator)(nil)

type windowManipulator struct {
	width, height int
	wm            browserapi.WindowManager
	m             browserapi.Notifications
	title         string
}

func newWindowManipulator(
	wm browserapi.WindowManager,
) *windowManipulator {
	return &windowManipulator{wm: wm}
}

func (w *windowManipulator) State() termutil.WindowState {
	return termutil.StateUnknown
}

func (w *windowManipulator) Minimise() {
}

func (w *windowManipulator) Maximise() {
}

func (w *windowManipulator) Restore() {
}

func (w *windowManipulator) SetTitle(title string) {
	w.title = title
}

func (w *windowManipulator) Position() (int, int) {
	return 0, 0
}

func (w *windowManipulator) SizeInPixels() (int, int) {
	panic("cannot determine size in pixels")
}

func (w *windowManipulator) CellSizeInPixels() (int, int) {
	panic("cannot determine size in pixels")
}

func (w *windowManipulator) SizeInChars() (int, int) {
	return w.height, w.width
}

func (w *windowManipulator) ResizeInPixels(int, int) {
	panic("cannot resize")
}

func (w *windowManipulator) ResizeInChars(height, width int) {
	w.width = width
	w.height = height
}

func (w *windowManipulator) ScreenSizeInPixels() (int, int) {
	panic("cannot determine size in pixels")
}

func (w *windowManipulator) ScreenSizeInChars() (int, int) {
	panic("cannot determine size in pixels")
}

func (w *windowManipulator) Move(x, y int) {
	panic("cannot move window")
}

func (w *windowManipulator) IsFullscreen() bool {
	return false
}

func (w *windowManipulator) SetFullscreen(enabled bool) {
}

func (w *windowManipulator) GetTitle() string {
	return w.title
}

func (w *windowManipulator) SaveTitleToStack() {
}

func (w *windowManipulator) RestoreTitleFromStack() {
}

func (w *windowManipulator) ReportError(err error) {
	w.m.Notify(notifications.LevelError, "terminal: %s", err)
}
