package text

// Clipboard is the interface that wraps the basic Copy, Paste short-term data storage
// methods for editors to use multiple storage registers.
type Clipboard interface {
	Paste(registerID string) (ClipboardData, error)
	Copy(registerID string, data ClipboardData) error
}

// ClipboardData is the structure used for short-term data storage and/or data transfer
// via Clipboard's Copy/Paste operations.
type ClipboardData struct {
	Text     string
	Metadata interface{}
}

var (
	// DefaultRegisterID represents the program's default clipboard register to be used with
	// Clipboard's Copy/Paste operations.
	DefaultRegisterID string = " "

	// UnnamedRegisterID is an alias of DefaultRegisterID.
	UnnamedRegisterID = DefaultRegisterID
)

type inmemoryClipboard struct {
	data ClipboardData
}

// NewInMemoryClipboard returns a simple in-memory implementation of Clipboard.
func NewInMemoryClipboard() Clipboard {
	return &inmemoryClipboard{}
}

func (d *inmemoryClipboard) Paste(id string) (ret ClipboardData, err error) {
	return d.data, nil
}

func (d *inmemoryClipboard) Copy(id string, data ClipboardData) error {
	d.data = data
	return nil
}
