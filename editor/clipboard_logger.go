package editor

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"
)

type clipboardLogger struct {
	logger  *log.Logger
	other   Clipboard
	refType string
}

// NewLoggingClipboard returns an implementation of Clipboard which simply logs
// all calls to the underlying clipboard via the given logger.
func NewLoggingClipboard(logger *log.Logger, other Clipboard) Clipboard {
	c := new(clipboardLogger)
	c.other = other
	c.logger = logger
	c.refType = reflect.TypeOf(other).Elem().String()
	return c
}

func (c *clipboardLogger) fields(method string) log.Fields {
	return log.Fields{
		"address": fmt.Sprintf("%p", c.other),
		"type":    c.refType,
		"method":  method,
	}
}

func (c *clipboardLogger) Get() (str string, err error) {
	fields := c.fields("Get")

	str, err = c.other.Get()

	fields["string"] = fmt.Sprintf("%.10s", str)
	entry := c.logger.WithFields(fields)

	if err != nil {
		fields["error"] = err
		entry.Error()
		return
	}
	entry.Trace()

	return
}

func (c *clipboardLogger) Set(str string) (err error) {
	fields := c.fields("Set")
	fields["string"] = fmt.Sprintf("%.10s", str)

	err = c.other.Set(str)

	entry := c.logger.WithFields(fields)
	if err != nil {
		fields["error"] = err
		return
	}

	entry.Trace()
	return
}
