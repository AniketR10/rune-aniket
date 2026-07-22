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


package main

import (
	"context"
	"sync"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/ide/ideauthorizer"
	"unstable.build/go-tui/text"
)

type bootstrapFlagOverrides struct {
	httpAddress    string
	dataPath       string
	configPath     string
	websiteAddress string
}

func overrideBootstrapFlags(t *testing.T, o bootstrapFlagOverrides) func() {
	t.Helper()
	prevHTTP := *flagHTTPAddress
	prevData := *flagDataPath
	prevConfig := *flagConfigPath
	prevWebsite := *flagWebsiteAddress
	*flagHTTPAddress = o.httpAddress
	*flagDataPath = o.dataPath
	*flagConfigPath = o.configPath
	*flagWebsiteAddress = o.websiteAddress
	return func() {
		*flagHTTPAddress = prevHTTP
		*flagDataPath = prevData
		*flagConfigPath = prevConfig
		*flagWebsiteAddress = prevWebsite
	}
}

// newBootstrapPublishPump wires the publish-event hook the way runGUI
// does: a pumper goroutine runs EventInterrupt UserFuncs under the
// bootstrap locker. Register the returned stop func with t.Cleanup
// after the handler's Close cleanup so it runs before it (LIFO),
// matching the runtime's pump-then-handler teardown order.
//
// stop fences off further publishes before closing the pump channel:
// the prompt-frame shader keeps interrupting at its own FPS well past
// a fast test's lifetime, and a late publish into a closed channel
// would panic the suite.
func newBootstrapPublishPump(
	mu *sync.Mutex,
) (publish func(term.Event) bool, stop func()) {
	publishCh := make(chan term.Event, 256)
	var publishMu sync.Mutex
	stopped := false
	publish = func(ev term.Event) bool {
		publishMu.Lock()
		defer publishMu.Unlock()
		if stopped {
			return false
		}
		select {
		case publishCh <- ev:
			return true
		default:
			return false
		}
	}
	pumperDone := make(chan struct{})
	go debug.CapturePanicReport(func() {
		defer close(pumperDone)
		for ev := range publishCh {
			if ev.Type == term.EventInterrupt && ev.UserFunc != nil {
				mu.Lock()
				ev.UserFunc()
				mu.Unlock()
			}
		}
	})
	stop = func() {
		publishMu.Lock()
		stopped = true
		publishMu.Unlock()
		close(publishCh)
		<-pumperDone
	}
	return publish, stop
}

func testE2EExtensionsRunner(
	_ workspaceapi.URI,
	_ map[extensionapi.Permission]extension.ResourceRegistrar,
	_, _ string,
	_ browser.Notifications,
	_, _ schemeapi.Executor,
	_ extension.Grantor,
	_ text.Editor,
	_ ideauthorizer.PromptOpener,
	_ storageapi.Service,
	_ func(func()) bool,
) (extension.Runner, error) {
	return nopE2ERunner{}, nil
}

type nopE2ERunner struct{}

func (nopE2ERunner) Run(string, string, config.Config) error { return nil }
func (nopE2ERunner) Close() error                            { return nil }
func (n nopE2ERunner) WaitReady(ctx context.Context, id string) error {
	return nil
}
