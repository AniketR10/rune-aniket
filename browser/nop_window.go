package browser

type noopWindow struct{}

func (w noopWindow) SetContent(h Handler) error { return nil }
func (w noopWindow) Close() error               { return nil }
func (w noopWindow) onWindowClosed(fn func())   {}

// NopWindow returns a window that does nothing.
func NopWindow() Window {
	return noopWindow{}
}
