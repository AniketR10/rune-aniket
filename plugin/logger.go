package plugin

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/sirupsen/logrus"
	"github.com/ernestrc/blue/logging"
)

var pluginLogger logrus.Logger

func init() {
	pluginLogger = *logrus.New()
	SetLoggingOutput(ioutil.Discard)
	SetLoggingLevel(logrus.InfoLevel)
	SetLoggingFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		TimestampFormat: time.StampMilli,
	})
}

// Logger returns the global plugins logger.
func Logger() *logrus.Logger {
	return &pluginLogger
}

// SetLoggingOutput sets the logging output of all plugins to out.
// This should only be called from plugin host, if called outside of this package.
func SetLoggingOutput(out io.Writer) {
	pluginLogger.SetOutput(out)
}

// SetLoggingFormatter sets the logging formatter of all plugins to f.
// This should only be called from plugin host, if called outside of this package.
func SetLoggingFormatter(f logrus.Formatter) {
	pluginLogger.SetFormatter(f)
}

// SetLoggingLevel sets the logging level of all plugins to level.
// This should only be called from plugin host, if called outside of this package.
func SetLoggingLevel(level logrus.Level) {
	pluginLogger.SetLevel(level)
}

// satisfies hclog.Logger by using a logrus.Logger
type hcloggerLogrus struct {
	logger *logrus.Logger
	fields logrus.Fields
}

// NewHCLogLogrus returns an instance of a struct that satisfies hclog.Logger by means
// of using a logrus.Logger.
func NewHCLogLogrus(logger *logrus.Logger) hclog.Logger {
	return &hcloggerLogrus{
		logger: logger,
		fields: logrus.Fields{},
	}
}

// Args are alternating key, val pairs
// keys must be strings
// vals can be any type, but display is implementation specific
// Emit a message and key/value pairs at the TRACE level
func (l *hcloggerLogrus) Trace(msg string, args ...interface{}) {
	lf := l.With(args).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Trace(msg)
}

// Emit a message and key/value pairs at the DEBUG level
func (l *hcloggerLogrus) Debug(msg string, args ...interface{}) {
	lf := l.With(args).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Debug(msg)
}

// Emit a message and key/value pairs at the INFO level
func (l *hcloggerLogrus) Info(msg string, args ...interface{}) {
	lf := l.With(args).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Info(msg)
}

// Emit a message and key/value pairs at the WARN level
func (l *hcloggerLogrus) Warn(msg string, args ...interface{}) {
	lf := l.With(args).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Warn(msg)
}

// Emit a message and key/value pairs at the ERROR level
func (l *hcloggerLogrus) Error(msg string, args ...interface{}) {
	lf := l.With(args).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Error(msg)
}

// Indicate if TRACE logs would be emitted. This and the other Is* guards
// are used to elide expensive logging code based on the current level.
func (l *hcloggerLogrus) IsTrace() bool {
	return l.logger.IsLevelEnabled(logrus.TraceLevel)
}

// Indicate if DEBUG logs would be emitted. This and the other Is* guards
func (l *hcloggerLogrus) IsDebug() bool {
	return l.logger.IsLevelEnabled(logrus.DebugLevel)
}

// Indicate if INFO logs would be emitted. This and the other Is* guards
func (l *hcloggerLogrus) IsInfo() bool {
	return l.logger.IsLevelEnabled(logrus.InfoLevel)
}

// Indicate if WARN logs would be emitted. This and the other Is* guards
func (l *hcloggerLogrus) IsWarn() bool {
	return l.logger.IsLevelEnabled(logrus.WarnLevel)
}

// Indicate if ERROR logs would be emitted. This and the other Is* guards
func (l *hcloggerLogrus) IsError() bool {
	return l.logger.IsLevelEnabled(logrus.ErrorLevel)
}

// Creates a sublogger that will always have the given key/value pairs
func (l *hcloggerLogrus) With(args ...interface{}) hclog.Logger {
	retFields := logrus.Fields{}

	for k, v := range l.fields {
		retFields[k] = v
	}

	ret := &hcloggerLogrus{
		logger: l.logger,
		fields: retFields,
	}

	var key string
	var ok bool
	for i, arg := range args {
		if i%2 == 0 {
			if key, ok = arg.(string); !ok {
				key = fmt.Sprintf("%+v", key)
			}
		} else {
			ret.fields[key] = arg
		}
	}
	return ret
}

// Create a logger that will prepend the name string on the front of all messages.
// If the logger already has a name, the new value will NOT be appended to the current
// name, instead it will substitue it. This does not conform to the original hclog.Logger
// interface requirements.
func (l *hcloggerLogrus) Named(name string) hclog.Logger {
	return l.With(logging.KeyClass, name)
}

// Create a logger that will prepend the name string on the front of all messages.
func (l *hcloggerLogrus) ResetNamed(name string) hclog.Logger {
	return l.With(logging.KeyClass, name)
}

// Updates the level. This should affect all sub-loggers as well. If an
// implementation cannot update the level on the fly, it should no-op.
func (l *hcloggerLogrus) SetLevel(level hclog.Level) {
	var logrusLevel logrus.Level
	switch level {
	case hclog.NoLevel:
		logrusLevel = logrus.ErrorLevel
	case hclog.Trace:
		logrusLevel = logrus.TraceLevel
	case hclog.Debug:
		logrusLevel = logrus.DebugLevel
	case hclog.Info:
		logrusLevel = logrus.InfoLevel
	case hclog.Warn:
		logrusLevel = logrus.WarnLevel
	case hclog.Error:
		logrusLevel = logrus.ErrorLevel
	}
	l.logger.SetLevel(logrusLevel)
}

func (l *hcloggerLogrus) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(l.logger.Writer(), "", log.LstdFlags)
}
