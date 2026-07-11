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

package ide

import (
	"sync"
	"testing"
)

// TestLazyParserConcurrentInit reproduces the data race where markdown code
// blocks each spawn a goroutine that calls Highlight, all racing to lazily
// construct the shared parser. parser() must construct exactly one instance
// and return the same value to every caller under -race.
func TestLazyParserConcurrentInit(t *testing.T) {
	lp := &lazyParser{root: &workspaceManagerHandler{}}

	const goroutines = 64
	var start sync.WaitGroup
	start.Add(1)
	var done sync.WaitGroup
	results := make([]any, goroutines)
	for i := range results {
		done.Add(1)
		go func(i int) {
			defer done.Done()
			start.Wait()
			results[i] = lp.parser()
		}(i)
	}
	start.Done()
	done.Wait()

	first := results[0]
	if first == nil {
		t.Fatal("parser() returned nil")
	}
	for i, got := range results {
		if got != first {
			t.Fatalf("goroutine %d got a different parser instance: %v != %v", i, got, first)
		}
	}
}
