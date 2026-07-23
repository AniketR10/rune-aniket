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

package vctrl

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/debug"
)

func TestDiffToLocationList(t *testing.T) {
	suite := []struct {
		description string
		diff        FileDiff
		expected    []textapi.Location
	}{
		{"empty FileDiff returns empty locations", FileDiff{}, []textapi.Location{}},
		{"converts an add operation", FileDiff{Hunks: []Hunk{
			{NewStartLine: 1, NewLines: 1},
		}}, []textapi.Location{
			{
				From: term.Coordinates{},
				To:   term.Coordinates{Y: 1},
				Attr: term.Attributes{Fg: term.ColorGreen},
				Icon: "+",
			},
		}},
		{"converts a delete operation", FileDiff{Hunks: []Hunk{
			{OrigStartLine: 1, OrigLines: 1},
		}}, []textapi.Location{
			{
				From: term.Coordinates{},
				To:   term.Coordinates{Y: 0, X: 1},
				Attr: term.Attributes{Fg: term.ColorRed},
				Icon: "-",
			},
		}},
		{"converts a replace operation into an add", FileDiff{Hunks: []Hunk{
			{OrigStartLine: 1, OrigLines: 1, NewStartLine: 1, NewLines: 1},
		}}, []textapi.Location{
			{
				From: term.Coordinates{},
				To:   term.Coordinates{Y: 1},
				Attr: term.Attributes{Fg: term.ColorGreen},
				Icon: "+",
			},
		}},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			actual := test.diff.LocationList(
				term.Attributes{Fg: term.ColorRed}, term.Attributes{Fg: term.ColorGreen})
			locs := make([]textapi.Location, 0)
			for loc, ok := actual.Current(); ok; loc, ok = actual.Next() {
				locs = append(locs, loc)
			}
			assert.Equal(t, test.expected, locs)
		})
	}
}

func TestCachePurgeDoesNotBlockOnDiff(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///workspace/file.go")
	require.NoError(t, err)

	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	oldDiff := FileDiff{OrigName: "old"}
	newDiff := FileDiff{OrigName: "new"}
	root := cacheTestService{diff: func(
		context.Context, workspaceapi.URI,
	) (FileDiff, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			return oldDiff, nil
		}
		return newDiff, nil
	}}
	cache := NewCache(root)

	firstResult := make(chan FileDiff, 1)
	go debug.CapturePanicReport(func() {
		diff, _ := cache.Diff(context.Background(), uri)
		firstResult <- diff
	})
	<-started

	purged := make(chan struct{})
	go debug.CapturePanicReport(func() {
		cache.Purge()
		close(purged)
	})

	blocked := false
	select {
	case <-purged:
	case <-time.After(time.Second):
		blocked = true
	}
	close(release)
	if blocked {
		<-purged
	}
	require.False(t, blocked, "Purge blocked behind an in-flight Diff")
	require.Equal(t, oldDiff, <-firstResult)

	diff, err := cache.Diff(context.Background(), uri)
	require.NoError(t, err)
	require.Equal(t, newDiff, diff)
	require.EqualValues(t, 2, calls.Load())

	diff, err = cache.Diff(context.Background(), uri)
	require.NoError(t, err)
	require.Equal(t, newDiff, diff)
	require.EqualValues(t, 2, calls.Load())
}

func TestCacheDiffCachesErrors(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///workspace/file.go")
	require.NoError(t, err)

	wantDiff := FileDiff{OrigName: "partial"}
	wantErr := errors.New("diff failed")
	var calls atomic.Int32
	cache := NewCache(cacheTestService{diff: func(
		context.Context, workspaceapi.URI,
	) (FileDiff, error) {
		calls.Add(1)
		return wantDiff, wantErr
	}})

	for range 2 {
		diff, err := cache.Diff(context.Background(), uri)
		require.Equal(t, wantDiff, diff)
		require.ErrorIs(t, err, wantErr)
	}
	require.EqualValues(t, 1, calls.Load())

	cache.Purge()
	diff, err := cache.Diff(context.Background(), uri)
	require.Equal(t, wantDiff, diff)
	require.ErrorIs(t, err, wantErr)
	require.EqualValues(t, 2, calls.Load())
}

type cacheTestService struct {
	diff func(context.Context, workspaceapi.URI) (FileDiff, error)
}

func (s cacheTestService) Diff(
	ctx context.Context, file workspaceapi.URI,
) (FileDiff, error) {
	return s.diff(ctx, file)
}

func (cacheTestService) CurrentCommit(context.Context, workspaceapi.URI) (string, error) {
	return "", nil
}

func (cacheTestService) ShortRef(context.Context, workspaceapi.URI) (string, error) {
	return "", nil
}

func (cacheTestService) RemoteURL(
	context.Context, workspaceapi.URI, string,
) (string, error) {
	return "", nil
}

func (cacheTestService) ListRemotes(context.Context, workspaceapi.URI) ([]string, error) {
	return nil, nil
}

func (cacheTestService) RelPath(context.Context, string) (string, error) {
	return "", nil
}
