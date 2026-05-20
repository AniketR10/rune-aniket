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


// Package main is a tiny program with a deliberate off-by-one
// bug used by the debugshell e2e tests. Sum iterates one past
// n, so Sum(5) returns 21 instead of 15. The bug is a clean
// breakpoint target: stop inside Sum, inspect i and total at
// the boundary.
//
// When invoked with the single argument "wait", main loops
// calling Sum forever (with a small sleep between iterations)
// so the attach test has time to spawn the process, connect
// dlv to it, and reliably hit a breakpoint inside Sum.
package main

import (
	"fmt"
	"os"
	"time"
)

// Sum is intentionally buggy: it iterates one past n.
func Sum(n int) int {
	total := 0

	for i := 1; i <= n+1; i++ {
		total += i
	}
	return total
}

// Other is a sibling helper that intentionally reuses the same
// local names (n, total) so the e2e tests can verify scope
// filtering: when the debuggee is stopped inside Sum, the
// references to n/total inside Other must NOT be highlighted
// as in-scope variables.
func Other(n int) int {
	total := n * 2
	return total
}

// AlwaysFalse exists only as a breakpoint target whose body
// contains a single literal-only statement (`return false`).
// Tree-sitter emits no identifier captures for this line, so
// the regression test for RUNE-177 can pin down that the
// breakpoint still binds.
func AlwaysFalse() bool {
	return false
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "wait" {
		for {
			_ = Sum(5)
			_ = Other(3)
			_ = AlwaysFalse()
			time.Sleep(50 * time.Millisecond)
		}
	}
	fmt.Println("Sum:", Sum(5))
}
