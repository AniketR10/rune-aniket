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

func (c *clipboardLogger) Get() (data Paste, err error) {
	fields := c.fields("Get")

	data, err = c.other.Get()

	fields["string"] = fmt.Sprintf("%.10s", data.Data)
	fields["metadata"] = fmt.Sprintf("%+v", data.Metadata)
	entry := c.logger.WithFields(fields)

	if err != nil {
		fields["error"] = err
		entry.Error()
		return
	}
	entry.Trace()

	return
}

func (c *clipboardLogger) Set(data Paste) (err error) {
	fields := c.fields("Set")
	fields["string"] = fmt.Sprintf("%.10s", data.Data)
	fields["metadata"] = fmt.Sprintf("%+v", data.Metadata)

	err = c.other.Set(data)

	entry := c.logger.WithFields(fields)
	if err != nil {
		fields["error"] = err
		return
	}

	entry.Trace()
	return
}
