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

package syntax

import (
	"context"
	"errors"

	"github.com/unstablebuild/blue/iterator"
)

// State represents the state of the syntax tree.
type State struct {
	Progress    float64
	ParserError string
	LangID      string
	Closed      bool
	Highlights  bool
	Folds       bool
	Indents     bool
}

type waitStateIterator struct {
	ready chan struct{}
	err   error
	tree  *Tree
	iter  iterator.Iterator[State]
}

func newWaitStateIterator(t *Tree, ch chan struct{}) *waitStateIterator {
	return &waitStateIterator{
		tree:  t,
		ready: ch,
	}
}

func (f *waitStateIterator) Next(ctx context.Context) (State, bool) {
	if f.iter == nil {
		select {
		case <-f.ready:
		case <-ctx.Done():
			f.err = ctx.Err()
			return State{}, false
		}
		f.tree.mu.Lock()
		f.iter = newReadyStateIterator(f.tree)
		f.tree.mu.Unlock()
	}
	return f.iter.Next(ctx)
}

func (f *waitStateIterator) Err() error {
	if f.err != nil {
		return f.err
	}
	if f.iter == nil {
		f.tree.mu.Lock()
		defer f.tree.mu.Unlock()
		if f.tree.tree == nil {
			return errors.New(f.tree.currState.ParserError)
		}
		if f.tree.closed {
			return errors.New("syntax tree closed")
		}
		return nil
	}
	return f.iter.Err()
}

func (f *waitStateIterator) Close() error {
	return nil
}

type readyStateIterator struct {
	t    *Tree
	err  error
	next chan State
}

func newReadyStateIterator(t *Tree) iterator.Iterator[State] {
	ch := make(chan State, 1)
	t.statesubs[ch] = struct{}{}

	ch <- t.currState
	return &readyStateIterator{
		t:    t,
		next: ch,
	}
}

func (f *readyStateIterator) Next(ctx context.Context) (State, bool) {
	select {
	case state, ok := <-f.next:
		return state, ok
	case <-ctx.Done():
		f.err = ctx.Err()
		return State{}, false
	}
}

func (f *readyStateIterator) Err() error {
	return f.err
}

func (f *readyStateIterator) Close() error {
	f.t.mu.Lock()
	defer f.t.mu.Unlock()
	delete(f.t.statesubs, f.next)
	return nil
}
