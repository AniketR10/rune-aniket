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

package langext

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
)

// Initializer wires editor open events to project-root discovery and
// dedupes language-server bring-up so InitRoot runs at most once per
// root. It is safe for concurrent use.
type Initializer struct {
	baseCtx context.Context
	fs      workspaceapi.FileSystem
	editor  textapi.Editor
	cfg     ProjectConfig

	mu           sync.Mutex
	initialized  map[string]Root
	initializing map[string]struct{}
}

// NewInitializer returns an Initializer for cfg that discovers roots
// under the editor's workspace via fs and brings them up through
// cfg.InitRoot. baseCtx must outlive the workspace session: event-driven
// bring-up runs under it rather than the per-event dispatch context,
// which the editor cancels as soon as Handle returns.
func NewInitializer(
	baseCtx context.Context, fs workspaceapi.FileSystem, editor textapi.Editor, cfg ProjectConfig,
) *Initializer {
	return &Initializer{
		baseCtx:      baseCtx,
		fs:           fs,
		editor:       editor,
		cfg:          cfg,
		initialized:  make(map[string]Root),
		initializing: make(map[string]struct{}),
	}
}

// Start subscribes the Initializer to editor open events. Call it once
// per workspace. It returns the subscription error, if any.
func (i *Initializer) Start() error {
	return i.editor.SubscribeEvents([]textapi.EventType{textapi.EventTypeOpen}, i)
}

// Handle implements textapi.EventHandler. It discovers the project root
// for an opened, language-matching file and kicks off a deduped
// background bring-up. It always returns false so the subscription stays
// active for later opens; the work happens off the calling goroutine
// under the Initializer's base context, so the editor is never blocked
// and the bring-up survives the per-event context being canceled.
func (i *Initializer) Handle(_ context.Context, ev textapi.Event) bool {
	if ev.Type != textapi.EventTypeOpen {
		return false
	}
	if i.cfg.FileMatch == nil || !i.cfg.FileMatch(ev.URI) {
		return false
	}

	wsRoot, err := i.fs.URI(".")
	if err != nil {
		slog.Warn("langext: resolve workspace root uri", "lang", i.cfg.LanguageID, "error", err)
		return false
	}

	root, found := FindProjectRoot(i.fs, wsRoot, ev.URI, i.cfg.Markers)
	if !found {
		return false
	}
	i.initializeAsync(i.baseCtx, root)
	return false
}

// InitializeAt eagerly brings up a specific root and returns the
// bring-up error, deduped against event-driven bring-up. It is used for
// the workspace-root project on startup. If the root is already
// initialized or in flight, it returns nil without running InitRoot
// again.
func (i *Initializer) InitializeAt(ctx context.Context, root Root) error {
	if !i.claim(root) {
		return nil
	}
	return i.run(ctx, root)
}

// Reinitialize re-runs InitRoot for every root brought up so far. It is
// used by consumers that must rebuild language servers after an
// out-of-band change (e.g. a Rust toolchain switch). Concurrent
// event-driven inits remain deduped: a root still initializing is
// skipped rather than run twice.
func (i *Initializer) Reinitialize(ctx context.Context) error {
	i.mu.Lock()
	roots := make([]Root, 0, len(i.initialized))
	for dir, root := range i.initialized {
		delete(i.initialized, dir)
		i.initializing[dir] = struct{}{}
		roots = append(roots, root)
	}
	i.mu.Unlock()

	var errs []error
	for _, root := range roots {
		if err := i.run(ctx, root); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// initializeAsync claims root and, if newly claimed, spawns the bring-up
// goroutine so the editor's event delivery is never blocked.
func (i *Initializer) initializeAsync(ctx context.Context, root Root) {
	if !i.claim(root) {
		return
	}
	go debug.CapturePanicReport(func() {
		if err := i.run(ctx, root); err != nil {
			slog.Warn("langext: init project root failed",
				"lang", i.cfg.LanguageID, "root", root.Dir, "error", err)
		}
	})
}

// claim marks root as in-flight and reports whether the caller won the
// claim. A root already initialized or initializing is not re-claimed.
func (i *Initializer) claim(root Root) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, done := i.initialized[root.Dir]; done {
		return false
	}
	if _, busy := i.initializing[root.Dir]; busy {
		return false
	}
	i.initializing[root.Dir] = struct{}{}
	return true
}

// run executes InitRoot for a claimed root and transitions its state:
// on success the root becomes initialized; on failure it is released so
// a later open can retry rather than wedging the language for the
// session.
func (i *Initializer) run(ctx context.Context, root Root) error {
	err := i.cfg.InitRoot(ctx, root)
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.initializing, root.Dir)
	if err != nil {
		return err
	}
	i.initialized[root.Dir] = root
	return nil
}
