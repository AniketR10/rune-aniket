package editor

// Clipboard is the basic interface that wraps the methods Get and Set, which emulate
// the behaviour of a system clibpboard.
type Clipboard interface {
	Get() (string, error)
	Set(string) error
}

type ephemeralClipboard struct {
	content string
}

// NewEphemeralClipboard returns a clipboard which uses program memory to store and retrieve data.
func NewEphemeralClipboard() Clipboard {
	c := new(ephemeralClipboard)
	return c
}

func (c *ephemeralClipboard) Get() (string, error) {
	return c.content, nil
}

func (c *ephemeralClipboard) Set(data string) error {
	c.content = data
	return nil
}
