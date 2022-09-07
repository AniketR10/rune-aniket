package debug

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

var (
	defaultLogger *log.Logger
	mu            sync.Mutex
)

// InitLogger initializes the default debug logger.
// Note that this should never run production quality code.
// Its purpose is to enable using a custom logger across packages.
func InitLogger(logger *log.Logger) {
	mu.Lock()
	defer mu.Unlock()

	defaultLogger = logger
}

// StandardLogger returns the default debug logger.
// Note that this should never run production quality code.
// Its purpose is to enable using a custom logger across packages.
func StandardLogger() *log.Logger {
	mu.Lock()
	defer mu.Unlock()

	if defaultLogger != nil {
		return defaultLogger
	}
	return log.StandardLogger()
}
