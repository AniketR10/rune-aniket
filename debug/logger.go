package debug

import (
	log "github.com/sirupsen/logrus"
)

var defaultLogger *log.Logger

// InitLogger initializes the default debug logger.
// Note that this should never run production quality code.
// Its purpose is to enable using a custom logger across packages.
func InitLogger(logger *log.Logger) {
	defaultLogger = logger
}

// StandardLogger returns the default debug logger.
// Note that this should never run production quality code.
// Its purpose is to enable using a custom logger across packages.
func StandardLogger() *log.Logger {
	if defaultLogger != nil {
		return defaultLogger
	}
	return log.StandardLogger()
}
