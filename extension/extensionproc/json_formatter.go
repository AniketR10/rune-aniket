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

package extensionproc

import (
	log "github.com/sirupsen/logrus"
)

type jsonFormatter struct {
	formatter *log.JSONFormatter
}

func newJSONFormatter() log.Formatter {
	return jsonFormatter{
		formatter: &log.JSONFormatter{
			// timestamp format expected by hclog
			TimestampFormat: "2006-01-02T15:04:05.000000Z07:00",
			FieldMap: log.FieldMap{
				log.FieldKeyTime: "@timestamp",
				log.FieldKeyMsg:  "@message",
				// log.FieldKeyLevel: "@level",
			},
		},
	}
}

func (j jsonFormatter) Format(entry *log.Entry) ([]byte, error) {
	if entry.Data == nil {
		entry.Data = make(log.Fields)
	}
	// map 'warning' (logrus) to 'warn' (hclog) or otherwise
	// warning logs are printed verbatim (json).
	levelStr := entry.Level.String()
	if entry.Level == log.WarnLevel {
		levelStr = "warn"
	}
	entry.Data["@level"] = levelStr
	return j.formatter.Format(entry)
}
