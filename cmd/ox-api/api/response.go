// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
)

const (
	httpCallType = "HTTPResponse"
)

// response is the common response field used by http handlers
type response[T any] struct {
	Message string `json:"Message"`
	Data    *T     `json:"Data,omitempty"`
}

type hasID interface {
	LoggingID() string
}

func writeResponse[T hasID](
	ctx context.Context, traceID trace.ID,
	attemptAt time.Time, w http.ResponseWriter,
	req *http.Request, r response[T],
	loggingKeyID string,
) {
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "account.accountHandler"},
		{Key: "Method", Value: req.Method},
		{Key: "URL", Value: req.URL.String()},
		{Key: "ResponseMessage", Value: r.Message},
	}
	if r.Data != nil {
		fields = append(fields,
			logging.Field{Key: loggingKeyID, Value: (*r.Data).LoggingID()})
	}
	data, err := json.Marshal(r)
	if err != nil {
		logging.LogResult(err, attemptAt, traceID, httpCallType, fields...)
		return
	}

	_, err = w.Write(data)
	logging.LogResult(err, attemptAt, traceID, httpCallType, fields...)
}
