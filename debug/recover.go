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

package debug

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/debug"
	"gopkg.in/yaml.v3"
)

// CapturePanicReportWith captures a panic in fn and writes to disk
// a yaml report. The boolean value returned indicates if fn run with no panics.
// If the value is false, the returned string indicates the fs location of the report.
// An error is returned if there was a panic but the report couldn't be stored.
func CapturePanicReportWith(dir, pkg, version string, run func()) (
	panicValue any, err error, ok bool,
) {
	defer func() {
		panicValue = recover()
		if panicValue == nil {
			return
		}
		report := debug.BuildCrashReport(pkg, version, panicValue)

		var data []byte
		data, err = yaml.Marshal(report)
		if err != nil {
			log.Errorf("yaml %v: %v", report, err)
			return
		}
		var f *os.File
		f, err = os.CreateTemp(ReportsDir, fmt.Sprintf("%s_crash_report_", Package))
		if err != nil {
			log.Errorf("temp file: %v", err)
			return
		}
		if _, err = f.Write(data); err != nil {
			log.Errorf("write to report %q: %v", f.Name(), err)
			return
		}
		log.Warnf("saved crash report file://%v", f.Name())
	}()

	run()
	ok = true
	return
}

// CapturePanicReport captures a panic with CapturePanicReportWith,
// and exits or simply returns if there was no panic in fn. It uses
// the compile-time variables Tag, Package and ReportsDir, so make
// sure they're injected at compile-time when using this helper.
func CapturePanicReport(fn func()) {
	defer func() {
		panicValue := recover()
		if panicValue == nil {
			return
		}
		defer panic(panicValue)
		report := debug.BuildCrashReport(Package, Tag, panicValue)

		var data []byte
		data, err := yaml.Marshal(report)
		if err != nil {
			log.Errorf("yaml %v: %v", report, err)
			return
		}
		var f *os.File
		f, err = os.CreateTemp(ReportsDir, fmt.Sprintf("%s_crash_report_", Package))
		if err != nil {
			log.Errorf("temp file: %v", err)
			return
		}
		if _, err = f.Write(data); err != nil {
			log.Errorf("write to report %q: %v", f.Name(), err)
			return
		}
		log.Warnf("saved crash report file://%v", f.Name())

	}()

	fn()
}
