package debug

import (
	"io/ioutil"
	"sync"

	log "github.com/sirupsen/logrus"
)

var (
	defaultLogger = log.New()
	mu            sync.Mutex
)

func init() {
	defaultLogger.Out = ioutil.Discard
	defaultLogger.Level = log.PanicLevel
}

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
	return defaultLogger
}
