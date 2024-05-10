package scanner

import (
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
)

type loggingDriverer struct {
	driver Driver
}

func WithLoggingDriver(p Driver) Driver {
	return loggingDriverer{driver: p}
}

func (p loggingDriverer) Print(r rune) {
	p.log(log.TraceLevel, "print %c", r)
	p.driver.Print(r)
}
func (p loggingDriverer) Execute(ch byte) {
	p.log(log.TraceLevel, "execute %c", ch)
	p.driver.Execute(ch)
}
func (p loggingDriverer) Hook(
	params [][]uint16, intermediates []byte,
	ignore bool, action rune,
) {
	p.log(log.TraceLevel, "hook params=%+v, len(intermediates)=%d, "+
		"ignore=%t, action=%c",
		params, len(intermediates), ignore, action)
	p.driver.Hook(params, intermediates, ignore, action)
}

func (p loggingDriverer) Put(ch byte) {
	p.log(log.TraceLevel, "put %c", ch)
	p.driver.Put(ch)
}
func (p loggingDriverer) Unhook() {
	p.log(log.TraceLevel, "unhook")
	p.driver.Unhook()
}
func (p loggingDriverer) OSCDispatch(params [][]byte, bellTerminated bool) {
	p.log(log.TraceLevel, "OSCDispatch params=%v, bell=%t", params, bellTerminated)
	p.driver.OSCDispatch(params, bellTerminated)
}
func (p loggingDriverer) CSIDispatch(
	params [][]uint16, intermediates []byte,
	ignore bool, action rune,
) {
	p.log(log.TraceLevel, "CSIDispatch params=%+v, len(intermediates)=%d, "+
		"ignore=%t, action=%c",
		params, len(intermediates), ignore, action)
	p.driver.CSIDispatch(params, intermediates, ignore, action)
}
func (p loggingDriverer) ESCDispatch(
	intermediates []byte, ignore bool, ch byte,
) {
	p.log(log.TraceLevel, "ESCDispatch len(intermediates)=%d, "+
		"ignore=%t, ch=%c",
		len(intermediates), ignore, ch)
	p.driver.ESCDispatch(intermediates, ignore, ch)
}

func (p loggingDriverer) log(level log.Level, msg string, args ...any) {
	log.WithField(logging.KeyClass, "scanner.Driver").
		Logf(level, msg, args...)
}
