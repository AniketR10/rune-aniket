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
	"runtime"

	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
)

var _ browser.Floating = (*floatingClientImpl)(nil)

type floatingClientImpl struct {
	conn          rpc.MuxConn
	cancelFn      func()
	client        browserapi.Handler
	fc            FloatingClient
	errorCh       chan error
	width, height int
}

func newFloatingClient(
	handler browserapi.Handler, fc FloatingClient,
	cancelFn func(), conn rpc.MuxConn,
) *floatingClientImpl {
	ret := &floatingClientImpl{
		client:   handler,
		fc:       fc,
		errorCh:  make(chan error),
		cancelFn: cancelFn,
		conn:     conn,
	}
	runtime.SetFinalizer(ret, func(*floatingClientImpl) {
		cancelFn()
		conn.Close()
		runtime.SetFinalizer(ret, nil)
	})
	return ret
}

func (f *floatingClientImpl) Resize(width, height int) {
	f.width = width
	f.height = width
	f.client.Resize(width, height)
	runtime.KeepAlive(f)
}
func (f *floatingClientImpl) Draw(w term.Writer) {
	f.client.Draw(w)
	runtime.KeepAlive(f)
}

func (f *floatingClientImpl) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = f.client.Handle(ev)
	runtime.KeepAlive(f)
	return exit, handled
}

func (f *floatingClientImpl) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	pos, style, show = f.client.Cursor()
	runtime.KeepAlive(f)
	return pos, style, show
}

func (f *floatingClientImpl) Man() tui.Manual {
	man := f.client.Man()
	runtime.KeepAlive(f)
	return man
}

func (f *floatingClientImpl) Dimensions() (width, height int) {
	req := DimensionsRequest{}
	ctx := context.Background()
	resp, err := f.fc.Dimensions(ctx, &req)
	runtime.KeepAlive(f)
	if err != nil {
		select {
		case f.errorCh <- err:
		default:
		}
		// best effort
		return f.width, f.height
	}
	return int(resp.GetWidth()), int(resp.GetHeight())
}

func (f *floatingClientImpl) Close() error {
	err := f.client.Close()
	f.cancelFn()
	// it might already be closed by conn monitoring
	_ = f.conn.Close()
	runtime.SetFinalizer(f, nil)
	return err
}
