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

package cell

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// countingSubscriber pairs each OnWillEdit with an OnDidEdit the way
// workspace.file does, where the imbalance leaks a WaitGroup counter.
type countingSubscriber struct {
	will int
	did  int
}

func (s *countingSubscriber) OnWillEdit(
	_ context.Context, _, _ term.Coordinates, _ string,
) {
	s.will++
}

func (s *countingSubscriber) OnDidEdit(
	_ context.Context, _, _ term.Coordinates, _ string,
) {
	s.did++
}

type panicEditor struct{}

func (panicEditor) Edit(
	_ context.Context, _, _ term.Coordinates, _ string,
) (from, to term.Coordinates, old string) {
	panic("out of bounds")
}

// TestPublisherBalancesSubscribersOnEditPanic asserts that a panicking
// Edit still closes the OnWillEdit/OnDidEdit pairing. Editor.Edit is
// documented to panic on an out-of-bounds range, and workspace.file
// tracks in-flight edits on a WaitGroup that Close waits on, so leaking
// the pairing wedges shutdown instead of letting the crash surface.
func TestPublisherBalancesSubscribersOnEditPanic(t *testing.T) {
	sub := new(countingSubscriber)
	p := newPublisher(panicEditor{})
	p.Subscribe(sub)

	require.Panics(t, func() {
		p.Edit(context.Background(), term.Coordinates{}, term.Coordinates{}, "a")
	})

	assert.Equal(t, 1, sub.will)
	assert.Equal(t, sub.will, sub.did,
		"every OnWillEdit must be closed by an OnDidEdit, even when Edit panics")
}

// TestPublisherNotifiesSubscribersOnceOnSuccess guards the panic-safety
// bookkeeping against double-notifying the happy path.
func TestPublisherNotifiesSubscribersOnceOnSuccess(t *testing.T) {
	sub := new(countingSubscriber)
	cells := new(rawCells)
	cells.init()
	p := newPublisher(cells)
	p.Subscribe(sub)

	p.Edit(context.Background(), term.Coordinates{}, term.Coordinates{}, "a")

	assert.Equal(t, 1, sub.will)
	assert.Equal(t, 1, sub.did)
}

// TestBufferBalancesSubscribersOnOutOfBoundsPanic drives the same
// guarantee through a real Buffer and the out-of-bounds delete that
// rawCells actually panics on, which is how a wedged editor left
// workspace.file waiting on an edit that never completed.
func TestBufferBalancesSubscribersOnOutOfBoundsPanic(t *testing.T) {
	b := NewBuffer()
	b.Init()
	b.WriteString("ab")

	sub := new(countingSubscriber)
	b.Subscribe(sub)

	require.Panics(t, func() {
		b.editor.Edit(context.Background(),
			term.Coordinates{}, term.Coordinates{X: 5}, "")
	})

	assert.NotZero(t, sub.will)
	assert.Equal(t, sub.will, sub.did,
		"a panicking edit must not leave an edit in flight")
}
