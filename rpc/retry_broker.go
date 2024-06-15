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
package rpc

import (
	"context"
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/retry"
)

type retryBroker struct {
	b MuxBroker
	s retry.Strategy
}

// WithRetryBroker wraps a MuxBroker with retry.Strategy retries.
func WithRetryBroker(b MuxBroker, strategy retry.Strategy) MuxBroker {
	return retryBroker{b: b, s: strategy}
}

func (b retryBroker) log(method string, attempt int, err error) {
	log.WithFields(log.Fields{
		logging.KeyClass: "rpc.retryBroker",
		"method":         method,
		"attempt":        strconv.Itoa(attempt),
		logging.KeyError: fmt.Sprintf("%v", err),
	}).Trace()
}

func (b retryBroker) NewChannel(tags ...string) (srv MuxServer, err error) {
	ctx := context.Background()
	var attempt int
	err = retry.Retry(ctx, b.s, func(ctx context.Context) (bool, error) {
		attempt++
		srv, err = b.b.NewChannel(tags...)
		b.log("NewChannel", attempt, err)
		return true, err
	})
	return
}

func (b retryBroker) DialChannel(ctx context.Context, addr string, tags ...string) (
	conn MuxConn, err error,
) {
	var attempt int
	err = retry.Retry(ctx, b.s, func(ctx context.Context) (bool, error) {
		attempt++
		conn, err = b.b.DialChannel(ctx, addr, tags...)
		b.log("DialChannel", attempt, err)
		return true, err
	})
	return
}

func (b retryBroker) Close() error {
	return b.b.Close()
}
