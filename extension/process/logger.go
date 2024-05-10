package process

import (
	"fmt"
	"io"

	"log"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
)

var extensionLogger logrus.Logger

func initExtensionLogger() {
	extensionLogger = *logrus.New()
	SetLoggingOutput(io.Discard)
	SetLoggingLevel(logrus.InfoLevel)
	SetLoggingFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		TimestampFormat: time.StampMilli,
	})
}

// Logger returns the global extensions logger.
func Logger() *logrus.Logger {
	return &extensionLogger
}

// SetLoggingOutput sets the logging output of all extensions to out.
// This should only be called from extension host, if called outside of this package.
func SetLoggingOutput(out io.Writer) {
	extensionLogger.SetOutput(out)
}

// SetLoggingFormatter sets the logging formatter of all extensions to f.
// This should only be called from extension host, if called outside of this package.
func SetLoggingFormatter(f logrus.Formatter) {
	extensionLogger.SetFormatter(f)
}

// SetLoggingLevel sets the logging level of all extensions to level.
// This should only be called from extension host, if called outside of this package.
func SetLoggingLevel(level logrus.Level) {
	extensionLogger.SetLevel(level)
}

// satisfies hclog.Logger by using a logrus.Logger
type hcloggerLogrus struct {
	extensionID string
	logger      *logrus.Logger
	fields      logrus.Fields
}

// newHCLogLogrus returns an instance of a struct that satisfies hclog.Logger by means
// of using a logrus.Logger.
func newHCLogLogrus(extensionID string, logger *logrus.Logger) hclog.Logger {
	return &hcloggerLogrus{
		extensionID: extensionID,
		logger:      logger,
		fields:      logrus.Fields{},
	}
}

// Args are alternating key, val pairs
// keys must be strings
// vals can be any type, but display is implementation specific
// Emit a message and key/value pairs at the TRACE level
func (l *hcloggerLogrus) Trace(msg string, args ...interface{}) {
	lf := l.With(args...).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Trace(msg)
}

func (l *hcloggerLogrus) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Trace:
		l.Trace(msg, args...)
	case hclog.Debug:
		l.Debug(msg, args...)
	case hclog.Info:
		l.Info(msg, args...)
	case hclog.Warn:
		l.Warn(msg, args...)
	case hclog.Error:
		l.Error(msg, args...)
	}
}

// Emit a message and key/value pairs at the DEBUG level
func (l *hcloggerLogrus) Debug(msg string, args ...interface{}) {
	lf := l.With(args...).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Debug(msg)
}

// Emit a message and key/value pairs at the INFO level
func (l *hcloggerLogrus) Info(msg string, args ...interface{}) {
	lf := l.With(args...).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Info(msg)
}

// Emit a message and key/value pairs at the WARN level
func (l *hcloggerLogrus) Warn(msg string, args ...interface{}) {
	lf := l.With(args...).(*hcloggerLogrus)
	l.logger.WithFields(lf.fields).Warn(msg)
}

// Emit a message and key/value pairs at the ERROR level
func (l *hcloggerLogrus) Error(msg string, args ...interface{}) {
	lf := l.With(args...).(*hcloggerLogrus)
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

	var key string
	var ok bool
	for i, arg := range args {
		if i%2 == 0 {
			key, ok = arg.(string)
			if !ok {
				key = fmt.Sprintf("%v", arg)
			}
		} else {
			if key == logging.KeyTimestamp {
				continue // let logger set this
			}
			retFields[key] = arg
		}
	}

	return &hcloggerLogrus{
		logger: l.logger,
		fields: retFields,
	}
}

// Create a logger that will prepend the name string on the front of all messages.
// If the logger already has a name, the new value will NOT be appended to the current
// name, instead it will substitue it. This does not conform to the original hclog.Logger
// interface requirements.
func (l *hcloggerLogrus) Named(name string) hclog.Logger {
	// ignore name, as it's the executable file name
	// and it's not helpful to distinguish different extensions
	// running in the same process.
	// l.With(logging.KeyThread, name)
	return l.With(logging.KeyThread, l.extensionID)
}

func (l *hcloggerLogrus) Name() string {
	v, ok := l.fields[logging.KeyThread]
	if !ok {
		return ""
	}
	vs, ok := v.(string)
	if !ok {
		return ""
	}
	return vs
}

// Create a logger that will prepend the name string on the front of all messages.
func (l *hcloggerLogrus) ResetNamed(name string) hclog.Logger {
	return l.With(logging.KeyThread, name)
}

// Edits the level. This should affect all sub-loggers as well. If an
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

func (l *hcloggerLogrus) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return l.logger.Writer()
}

func (l *hcloggerLogrus) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(l.logger.Writer(), "", log.LstdFlags)
}

func (l *hcloggerLogrus) ImpliedArgs() []interface{} {
	return nil
}
