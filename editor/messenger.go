package editor

// Messenger wraps the basic method SetMessage, which enables
// editor implementations to render a diagnostic or message.
type Messenger interface {
	SetMessage(msg string, args ...interface{}) error
}
