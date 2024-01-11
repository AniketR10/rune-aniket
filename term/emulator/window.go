package emulator

import (
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/notifications"
	termutil "unstable.build/go-tui/term/emulator/util"
)

var _ termutil.WindowManipulator = (*windowManipulator)(nil)

type windowManipulator struct {
	width, height int
	notifications browser.Notifications
	title         string
}

func newWindowManipulator(
	n browser.Notifications,
) *windowManipulator {
	return &windowManipulator{notifications: n}
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
	return w.height, w.width
}

func (w *windowManipulator) CellSizeInPixels() (int, int) {
	return 1, 1
}

func (w *windowManipulator) SizeInChars() (int, int) {
	return w.height, w.width
}

func (w *windowManipulator) ResizeInPixels(height int, width int) {
	w.width = width
	w.height = height
}

func (w *windowManipulator) ResizeInChars(height, width int) {
	w.width = width
	w.height = height
}

func (w *windowManipulator) ScreenSizeInPixels() (int, int) {
	return w.height, w.width
}

func (w *windowManipulator) ScreenSizeInChars() (int, int) {
	return w.height, w.width
}

func (w *windowManipulator) Move(x, y int) {
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
	w.notifications.Notify(notifications.LevelError, "terminal: %s", err)
}
