package browser

type noopWindow struct{}

func (w noopWindow) Content() (Handler, error)  { return nil, nil }
func (w noopWindow) SetContent(h Handler) error { return nil }
func (w noopWindow) Close() error               { return nil }
func (w noopWindow) onWindowClosed(fn func())   {}
func (w noopWindow) id() uint64                 { return 0 }

// NopWindow returns a window that does nothing.
func NopWindow() Window {
	return noopWindow{}
}
