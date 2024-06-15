// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
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
