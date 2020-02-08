package editor

// Paste represents the contents stored in a Clipboard (or a.k.a paste buffer)
type Paste struct {
	Data     string
	Metadata interface{}
}

// Clipboard is the basic interface that wraps the methods Get and Set, which emulate
// the behaviour of a system clibpboard.
type Clipboard interface {
	Get() (Paste, error)
	Set(Paste) error
}

type ephemeralClipboard struct {
	content Paste
}

// NewEphemeralClipboard returns a clipboard which uses program memory to store and retrieve data.
func NewEphemeralClipboard() Clipboard {
	c := new(ephemeralClipboard)
	return c
}

func (c *ephemeralClipboard) Get() (Paste, error) {
	return c.content, nil
}

func (c *ephemeralClipboard) Set(data Paste) error {
	c.content = data
	return nil
}
