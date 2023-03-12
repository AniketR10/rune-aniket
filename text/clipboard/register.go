package clipboard

var (
	// DefaultRegisterID represents the program's default clipboard register to be used with
	// Clipboard's Copy/Paste operations.
	DefaultRegisterID string = " "

	// UnnamedRegisterID is an alias of DefaultRegisterID.
	UnnamedRegisterID = DefaultRegisterID
)

// Register is the interface that wraps the basic Copy, Paste short-term data storage
// methods for editors to use multiple storage registers.
type Register interface {
	Paste(registerID string) (Data, error)
	Copy(registerID string, data Data) error
}

// Data is the structure used for short-term data storage and/or data transfer
// via Clipboard's Copy/Paste operations.
type Data struct {
	Text     string
	Metadata interface{}
}
