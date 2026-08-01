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

// Package openpanel shows the platform's native open dialog. On macOS
// it is backed by NSOpenPanel; on other platforms every request
// completes immediately as cancelled.
package openpanel

import (
	"strings"
	"sync"
)

// Options configure the open panel.
type Options struct {
	// Title is the panel's window title.
	Title string
	// Message is explanatory text displayed above the file browser.
	Message string
	// Prompt overrides the confirm button label ("Open" by default).
	Prompt string
	// Directories selects directories instead of files.
	Directories bool
	// Multiple allows selecting more than one entry.
	Multiple bool
}

var (
	mu      sync.Mutex
	nextTag = 1
	pending = map[int]func([]string){}
)

// Show opens the native open panel and invokes done with the selected
// paths once the user confirms. done(nil) signals cancellation, or
// platforms without a native panel. Show does not block: the panel is
// window-modal and done fires during regular event dispatch.
//
// It must be called on the main thread; done is invoked on the main
// thread as well.
func Show(opts Options, done func(paths []string)) {
	showPanel(register(done), opts)
}

// register stores done under a fresh tag so the platform completion
// can be routed back to it. Registration is single-use: finish removes
// the entry.
func register(done func([]string)) int {
	mu.Lock()
	defer mu.Unlock()
	tag := nextTag
	nextTag++
	pending[tag] = done
	return tag
}

// finish routes the platform completion for tag to its registered
// callback.
func finish(tag int, paths []string) {
	mu.Lock()
	done, ok := pending[tag]
	delete(pending, tag)
	mu.Unlock()

	if ok && done != nil {
		done(paths)
	}
}

// splitPaths decodes the NUL-joined path list produced by the platform
// layer. The empty payload signals cancellation and decodes to nil.
func splitPaths(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, "\x00")
}
