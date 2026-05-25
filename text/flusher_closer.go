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

package text

import (
	"context"
	"time"

	"github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/workspace"
)

var _ workspace.FlusherCloser = (*editorFlusherCloser)(nil)

// used to intercept calls to Close and Flush to dispatch
// corresponding events to subscribers.
type editorFlusherCloser struct {
	parent    *Component
	fc        workspace.FlusherCloser
	h         Handler
	uri       workspaceapi.URI
	buf       *cell.Buffer
	commands  []textapi.CommandManual
	lastFlush int
	reloading bool
}

func (c *editorFlusherCloser) OnWillEdit(
	ctx context.Context, start, end term.Coordinates, str string,
) {
}

func (c *editorFlusherCloser) OnDidEdit(
	ctx context.Context, from, to term.Coordinates, old string,
) {
	if c.reloading {
		return
	}
	c.parent.setDirtyFileAttr(c.uri, c.buf, c.lastFlush)
}

func (e *editorFlusherCloser) ForceFlush(ctx context.Context) (<-chan error, error) {
	inner, err := e.fc.ForceFlush(ctx)
	if err != nil {
		return nil, err
	}
	return e.wrapAndDispatch(inner, false, false), nil
}

func (e *editorFlusherCloser) LastFlush() time.Time {
	return e.fc.LastFlush()
}

func (e *editorFlusherCloser) Flush(ctx context.Context) (<-chan error, error) {
	inner, err := e.fc.Flush(ctx)
	if err != nil {
		return nil, err
	}
	return e.wrapAndDispatch(inner, false, false), nil
}

func (e *editorFlusherCloser) Reload(ctx context.Context) (<-chan error, error) {
	e.reloading = true
	inner, err := e.fc.Reload(ctx)
	if err != nil {
		e.reloading = false
		return nil, err
	}
	return e.wrapAndDispatch(inner, true, true), nil
}

func (e *editorFlusherCloser) wrapAndDispatch(
	inner <-chan error, skipOnErr, isReload bool,
) <-chan error {
	out := make(chan error, 1)
	go debug.CapturePanicReport(func() {
		err := <-inner
		doDispatch := err == nil || !skipOnErr
		if !doDispatch {
			if isReload {
				e.parent.config.ScheduleNextTick(func() {
					e.reloading = false
				})
			}
			out <- err
			close(out)
			return
		}
		e.parent.config.ScheduleNextTick(func() {
			_ = e.dispatchFlush()
			if isReload {
				e.reloading = false
			}
		})
		out <- err
		close(out)
	})
	return out
}

func (e *editorFlusherCloser) dispatchFlush() error {
	e.lastFlush = e.buf.Version()
	e.parent.log(log.TraceLevel, "flushed, new snapshot is at %d", e.lastFlush)
	return e.parent.dispatchFlush(e.uri, e.h)
}

func (e *editorFlusherCloser) Close() error {
	ev := textapi.Event{
		Type:     textapi.EventTypeClose,
		URI:      e.uri,
		Resource: e.h,
	}
	e.parent.DispatchEvent(ev)
	ret := e.fc.Close()
	for _, cmd := range e.commands {
		err := e.parent.fileRegistry.UnsubscribeCommandForFile(e.uri, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return ret
}
