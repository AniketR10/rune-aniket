package plugin

import (
	"io"
	"io/ioutil"

	hPlugin "github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
)

var pluginLogger *log.Logger

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var handshakeConfig = hPlugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "BASIC_PLUGIN",
	MagicCookieValue: "hello",
}

func init() {
	pluginLogger = log.New()
	pluginLogger.SetOutput(ioutil.Discard)
}

// SetLoggingOutput sets the logging output of all plugins to out.
func SetLoggingOutput(out io.Writer) {
	pluginLogger.SetOutput(out)
}

// SetLoggingFormatter sets the logging formatter of all plugins to f.
func SetLoggingFormatter(f log.Formatter) {
	pluginLogger.SetFormatter(f)
}

// SetLoggingLevel sets the logging level of all plugins to level.
func SetLoggingLevel(level log.Level) {
	pluginLogger.SetLevel(level)
}
