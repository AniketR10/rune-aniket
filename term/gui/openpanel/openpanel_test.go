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

package openpanel

import (
	"reflect"
	"testing"
)

func TestFinishRoutesByTag(t *testing.T) {
	var first, second [][]string
	firstTag := register(func(paths []string) { first = append(first, paths) })
	secondTag := register(func(paths []string) { second = append(second, paths) })

	finish(secondTag, []string{"/b"})
	finish(firstTag, []string{"/a1", "/a2"})

	if want := [][]string{{"/a1", "/a2"}}; !reflect.DeepEqual(first, want) {
		t.Errorf("first = %v, want %v", first, want)
	}
	if want := [][]string{{"/b"}}; !reflect.DeepEqual(second, want) {
		t.Errorf("second = %v, want %v", second, want)
	}
}

func TestFinishCancelPassesNil(t *testing.T) {
	var got []string
	called := false
	tag := register(func(paths []string) {
		called = true
		got = paths
	})

	finish(tag, nil)

	if !called {
		t.Fatal("done was not invoked on cancel")
	}
	if got != nil {
		t.Errorf("paths = %v, want nil", got)
	}
}

func TestFinishIsSingleUse(t *testing.T) {
	calls := 0
	tag := register(func([]string) { calls++ })

	finish(tag, nil)
	finish(tag, []string{"/late"})

	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestFinishUnknownTagIsIgnored(t *testing.T) {
	finish(0, nil)
	finish(1<<30, []string{"/x"})
}

func TestSplitPaths(t *testing.T) {
	tests := []struct {
		name   string
		joined string
		want   []string
	}{
		{name: "empty payload signals cancel", joined: "", want: nil},
		{name: "single path", joined: "/tmp/a.txt", want: []string{"/tmp/a.txt"}},
		{
			name:   "multiple paths",
			joined: "/tmp/a.txt\x00/tmp/dir b/c.txt",
			want:   []string{"/tmp/a.txt", "/tmp/dir b/c.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitPaths(tt.joined); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitPaths(%q) = %v, want %v", tt.joined, got, tt.want)
			}
		})
	}
}
