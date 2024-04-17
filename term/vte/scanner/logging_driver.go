package scanner

import (
	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
)

type logginDriver struct {
	driver Driver
}

func WithLoggingDriver(p Driver) Driver {
	return logginDriver{driver: p}
}

func (p logginDriver) Print(r rune) {
	p.log(log.TraceLevel, "print %c", r)
	p.driver.Print(r)
}
func (p logginDriver) Execute(ch byte) {
	p.log(log.TraceLevel, "execute %c", ch)
	p.driver.Execute(ch)
}
func (p logginDriver) Hook(
	params [][]uint16, intermediates []byte,
	ignore bool, action rune,
) {
	p.log(log.TraceLevel, "hook params=%+v, len(intermediates)=%d, "+
		"ignore=%t, action=%c",
		params, len(intermediates), ignore, action)
	p.driver.Hook(params, intermediates, ignore, action)
}

func (p logginDriver) Put(ch byte) {
	p.log(log.TraceLevel, "put %c", ch)
	p.driver.Put(ch)
}
func (p logginDriver) Unhook() {
	p.log(log.TraceLevel, "unhook")
	p.driver.Unhook()
}
func (p logginDriver) OSCDispatch(params [][]byte, bellTerminated bool) {
	p.log(log.TraceLevel, "OSCDispatch params=%v, bell=%t", params, bellTerminated)
	p.driver.OSCDispatch(params, bellTerminated)
}
func (p logginDriver) CSIDispatch(
	params [][]uint16, intermediates []byte,
	ignore bool, action rune,
) {
	p.log(log.TraceLevel, "CSIDispatch params=%+v, len(intermediates)=%d, "+
		"ignore=%t, action=%c",
		params, len(intermediates), ignore, action)
	p.driver.CSIDispatch(params, intermediates, ignore, action)
}
func (p logginDriver) ESCDispatch(
	intermediates []byte, ignore bool, ch byte,
) {
	p.log(log.TraceLevel, "ESCDispatch len(intermediates)=%d, "+
		"ignore=%t, ch=%c",
		len(intermediates), ignore, ch)
	p.driver.ESCDispatch(intermediates, ignore, ch)
}

func (p logginDriver) UnknownAction(
	action Action,
) {
	p.log(log.TraceLevel, "UnknownAction: %v", action)
	p.driver.UnknownAction(action)
}

func (p logginDriver) log(level log.Level, msg string, args ...any) {
	log.WithField(logging.KeyClass, "scanner.Driver").
		Logf(level, msg, args...)
}
