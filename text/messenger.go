package text

// Notifications wraps the basic method Notify, which enables
// editor implementations to render a diagnostic or message.
type Notifications interface {
	Notify(msg string, args ...interface{}) error
}
