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

// CapturePanicReportDir captures a panic in run and writes a yaml report for the given pkg and
// version to the given dir. See debug.CapturePanic for more details.
func CapturePanicReportDir(dir, pkg, version string, run func()) (bool, string, error) {
	ok, report := debug.CapturePanic(log.StandardLogger(), pkg, version, run)
	if ok {
		return ok, "", nil
	}
	data, err := yaml.Marshal(report)
	if err != nil {
		return false, "", fmt.Errorf("%w: marshal %v", err, report)
	}
	f, err := os.CreateTemp(dir, fmt.Sprintf("%s_crash_report_", pkg))
	if err != nil {
		return false, "", fmt.Errorf("%w: temp file %v", err, report)
	}
	if _, err := f.Write(data); err != nil {
		return false, "", fmt.Errorf("%w: write %v", err, report)
	}
	return false, f.Name(), nil
}
