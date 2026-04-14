// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestGrepFiles_boundsWalkdirWorkers(t *testing.T) {
	dir := t.TempDir()
	for i := range 12 {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "file"+string(rune('a'+i))+".txt"),
			[]byte("match\n"), 0o644,
		))
	}

	fs := &recordingLocalFS{
		localFS: localFS{root: dir},
		delay:   20 * time.Millisecond,
	}
	tool := NewGrepFiles(fs, dirURI(dir), NewFileTracker())

	result := tool.Execute(context.Background(), `{"pattern":"match"}`)
	require.False(t, result.IsError, result.Content)

	wantMax := max(min(runtime.NumCPU(), maxAgentWalkdirWorkers), 1)
	assert.LessOrEqual(t, int(fs.maxConcurrent.Load()), wantMax)
}

type recordingLocalFS struct {
	localFS
	delay time.Duration

	active        atomic.Int64
	maxConcurrent atomic.Int64
}

func (f *recordingLocalFS) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	active := f.active.Add(1)
	for {
		maxConcurrent := f.maxConcurrent.Load()
		if active <= maxConcurrent || f.maxConcurrent.CompareAndSwap(maxConcurrent, active) {
			break
		}
	}
	defer f.active.Add(-1)

	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.localFS.OpenFile(path, flag, mode)
}
