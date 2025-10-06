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

package syntax

import (
	"context"

	"github.com/ernestrc/logd-go/logging"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

var _ text.EventHandler = (*Delegate)(nil)

// Delegate represents a collection of syntax trees with one to one mapping with files,
// powered by tree-sitter. It satisfies text.EventHandler, and it's the primary way
// how it should be integrated.
type Delegate struct {
	ed            text.Editor
	pkg           PkgManager
	interrupter   term.Interrupter
	notifications browserapi.Notifications
	config        Config

	ctx       context.Context
	cancelCtx func()
	trees     map[string]*tree
}

// NewDelegate allocates storage for a new Delegate and initializes it.
// It also returns a list of events that this Delegate is interested in
// subscribing to.
func NewDelegate(
	pkg PkgManager, n browserapi.Notifications,
	ed text.Editor, interrupter term.Interrupter, config Config,
) (*Delegate, []textapi.EventType) {
	if config.ScheduleNextTick == nil {
		panic("nil schedule function")
	}
	ret := new(Delegate)
	ret.pkg = pkg
	ret.interrupter = interrupter
	ret.trees = make(map[string]*tree)
	ret.notifications = n
	ret.ed = ed
	ret.config = config
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	return ret, []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeFlush,
		textapi.EventTypeEdit,
	}
}

// PkgManager abstracts a subset of idepkg.Manager for a Delegate.
type PkgManager interface {
	LibDir(ctx context.Context, pkgID string) iterator.Iterator[string]
}

// Handle satisfies text.EventHandler.
func (t *Delegate) Handle(ctx context.Context, ev textapi.Event) (exit bool) {
	uri := ev.URI.String()

	t.log(log.TraceLevel, "processing event %s for file %s", ev.Type, uri)

	switch ev.Type {
	case textapi.EventTypeOpen:
		t.trees[uri] = t.newTree(t.interrupter, ev.URI, ev.Resource.(text.Handler), t.pkg, t.config)
	case textapi.EventTypeClose:
		tree, ok := t.trees[uri]
		if ok {
			tree.Close()
		}
		delete(t.trees, uri)
	case textapi.EventTypeFlush:
		tree, ok := t.trees[uri]
		if !ok {
			t.log(log.WarnLevel, "tree for file %q not found", uri)
			return
		}
		tree.flush(ev)
	case textapi.EventTypeEdit:
		tree, ok := t.trees[uri]
		if !ok {
			t.log(log.WarnLevel, "tree for file %q not found", uri)
			return
		}
		tree.edit(ev)
	}
	return
}

// Close closes all pending requests and associated resources.
func (t *Delegate) Close() error {
	t.cancelCtx()
	return nil
}

func (t *Delegate) newTree(
	interrupter term.Interrupter,
	uri workspaceapi.URI, handler text.Handler,
	pkg PkgManager, config Config,
) *tree {
	ret := new(tree)
	ret.interrupter = interrupter
	ret.pkg = pkg
	ret.config = config
	ret.ed = t.ed
	ret.notifications = t.notifications
	ret.handler = handler
	ret.view = t.ed.CellView(handler)
	ret.uri = uri

	go debug.CapturePanicReport(func() {
		ret.downloadFiles(t.ctx)
	})
	return ret
}

func (t *Delegate) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "stree.Delegate").Logf(level, msg, args...)
}
