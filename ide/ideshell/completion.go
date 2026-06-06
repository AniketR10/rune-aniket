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

package ideshell

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// capturedCompletion records the prefix and candidates that the
// inputbox queried via the WordCompleter on a tab press. It is
// produced by completionShim and consumed by Handler to seed the
// fuzzy completion overlay.
type capturedCompletion struct {
	// prefix is the partial word at the cursor that the user has
	// already typed; replacing it with a candidate restores the
	// rest of the line untouched.
	prefix     string
	candidates []string
}

// completionShim wraps the user-facing repl.CommandHandler so that
// the SDK inputbox's tab handler sees zero candidates whenever the
// underlying handler returns more than one match. The shim records
// the original candidates so Handler can render them in a search.List
// overlay instead of cycling through them inline.
//
// HandleCommand is delegated unmodified, since command execution does
// not interact with completion.
type completionShim struct {
	underlying repl.CommandHandler

	captured capturedCompletion
	hasCap   bool
	// lastPrefix is the prefix that was active when the most recent
	// completion overlay was opened. Handler reads it on accept to
	// know how many runes to delete before inserting the candidate.
	lastPrefix string
	// inFlight counts commands whose output iterator the inner repl
	// has not yet closed. It is bumped here rather than read from the
	// SDK because repl.Handler exposes no "is a command running"
	// accessor. Mutated from both the event-loop goroutine (dispatch)
	// and the dispatch goroutine (Close), so it must be atomic.
	inFlight atomic.Int64
}

func (s *completionShim) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	iter, err := s.underlying.HandleCommand(ctx, cmd, pw)
	if err != nil {
		return nil, err
	}
	s.inFlight.Add(1)
	return &trackedIterator{Iterator: iter, shim: s}, nil
}

// running reports whether a dispatched command's output iterator is
// still open. <c-l> consults this to avoid clearing (and thereby
// aborting) a command that is still producing output.
func (s *completionShim) running() bool {
	return s.inFlight.Load() > 0
}

// trackedIterator decrements the shim's in-flight counter when the
// inner repl closes the command's output iterator, which it does once
// the iterator is fully drained.
type trackedIterator struct {
	iterator.Iterator[component.Responsive]
	shim *completionShim
	done atomic.Bool
}

func (t *trackedIterator) Close() error {
	if t.done.CompareAndSwap(false, true) {
		t.shim.inFlight.Add(-1)
	}
	return t.Iterator.Close()
}

// Complete proxies the call to the underlying handler and intercepts
// the result. When more than one candidate is returned we keep them
// for the caller (the IDE shell Handler) and return an empty iterator
// so the SDK inputbox does not enter its own completion mode.
func (s *completionShim) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	iter, err := s.underlying.Complete(ctx, cmd, args)
	if err != nil {
		return nil, err
	}
	candidates, err := iterator.ToSlice(ctx, iter)
	if err != nil {
		return nil, err
	}
	if len(candidates) <= 1 {
		return iterator.FromSlice(candidates), nil
	}
	s.captured = capturedCompletion{
		prefix:     completionPrefix(cmd, args),
		candidates: candidates,
	}
	s.hasCap = true
	return iterator.FromSlice[string](nil), nil
}

// reset clears any previously-captured completion state. It is called
// before forwarding a tab event so that a stale capture from an
// earlier press cannot leak into the current decision.
func (s *completionShim) reset() {
	s.captured = capturedCompletion{}
	s.hasCap = false
}

// consume returns and clears the most recent capture if any.
func (s *completionShim) consume() (capturedCompletion, bool) {
	if !s.hasCap {
		return capturedCompletion{}, false
	}
	c := s.captured
	s.captured = capturedCompletion{}
	s.hasCap = false
	return c, true
}

// completionPrefix returns the partial word at the cursor that the
// inputbox is asking the WordCompleter to extend. It mirrors the
// shape of repl.Handler.makeCompleter: when args is non-empty, the
// last element is the partial word; when args is empty (or nil) the
// command itself is the partial word. A trailing-space scenario is
// expressed as args[len-1] == "" by the SDK, which is the empty
// prefix we want.
func completionPrefix(cmd string, args []string) string {
	if len(args) > 0 {
		return args[len(args)-1]
	}
	// args == nil means command-name completion; the partial word is
	// the command itself.
	if strings.ContainsAny(cmd, " \t") {
		// Defensive: shouldn't happen for command-name completion,
		// but if it does, just take the trailing fragment.
		idx := strings.LastIndexAny(cmd, " \t")
		return cmd[idx+1:]
	}
	return cmd
}
