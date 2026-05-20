// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
	"net"
	"net/http"
	// pprof handlers register themselves on http.DefaultServeMux on import.
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// StartPProfOnSignal installs a SIGUSR1 handler that, on the first
// signal, starts a pprof HTTP server bound to a random localhost port
// and logs the listening address at info level so the caller can find
// it. Subsequent SIGUSR1 signals are ignored.
func StartPProfOnSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go CapturePanicReport(func() {
		<-ch
		signal.Stop(ch)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Errorf("pprof listen: %v", err)
			return
		}
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
		log.Infof("pprof server listening on http://%s/debug/pprof/", ln.Addr())
		if err := http.Serve(ln, nil); err != http.ErrServerClosed {
			log.Errorf("pprof serve: %v", err)
		}
	})
}
