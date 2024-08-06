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
	context "context"

	log "github.com/sirupsen/logrus"
)

type loggingBroker struct {
	root   MuxBroker
	logger *log.Logger
}

// LoggingBroker wraps other to provide trace-level logging.
func LoggingBroker(other MuxBroker, logger *log.Logger) MuxBroker {
	return loggingBroker{root: other, logger: logger}
}

func (b loggingBroker) NewChannel(tags ...string) (srv MuxServer, err error) {
	srv, err = b.root.NewChannel(tags...)
	b.logger.Tracef("loggingBroker: NewChannel(%v): %v %v", tags, srv, err)
	return
}

func (b loggingBroker) DialChannel(ctx context.Context, addr string, tags ...string) (conn MuxConn, err error) {
	conn, err = b.root.DialChannel(ctx, addr, tags...)
	b.logger.Tracef("loggingBroker: DialChannel(%s, %v): %v %v",
		addr, tags, conn, err)
	return
}

func (b loggingBroker) Close() error {
	err := b.root.Close()
	b.logger.Tracef("loggingBroker: Close(): %v", err)
	return err
}
