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

package record

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatcherMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		fields  map[string]any
		payload map[string]any
		want    bool
	}{
		{
			name:    "exact string match",
			fields:  map[string]any{"msg": "hello"},
			payload: map[string]any{"msg": "hello", "level": float64(2)},
			want:    true,
		},
		{
			name:    "exact string mismatch",
			fields:  map[string]any{"msg": "hello"},
			payload: map[string]any{"msg": "bye"},
			want:    false,
		},
		{
			name:    "absent field",
			fields:  map[string]any{"msg": "hello"},
			payload: map[string]any{"level": float64(2)},
			want:    false,
		},
		{
			name:    "nil payload",
			fields:  map[string]any{"msg": "hello"},
			payload: nil,
			want:    false,
		},
		{
			name:    "number normalization int vs float",
			fields:  map[string]any{"level": 2},
			payload: map[string]any{"level": float64(2)},
			want:    true,
		},
		{
			name:    "dotted path",
			fields:  map[string]any{"range.start.line": 3},
			payload: map[string]any{"range": map[string]any{"start": map[string]any{"line": float64(3)}}},
			want:    true,
		},
		{
			name:    "dotted path through non-map",
			fields:  map[string]any{"range.start": 3},
			payload: map[string]any{"range": "nope"},
			want:    false,
		},
		{
			name:   "nested map subset",
			fields: map[string]any{"range": map[string]any{"start": map[string]any{"line": 3}}},
			payload: map[string]any{"range": map[string]any{
				"start": map[string]any{"line": float64(3), "col": float64(9)},
				"end":   map[string]any{"line": float64(4)},
			}},
			want: true,
		},
		{
			name:    "list exact match",
			fields:  map[string]any{"args": []any{"a", "b"}},
			payload: map[string]any{"args": []any{"a", "b"}},
			want:    true,
		},
		{
			name:    "list length mismatch",
			fields:  map[string]any{"args": []any{"a"}},
			payload: map[string]any{"args": []any{"a", "b"}},
			want:    false,
		},
		{
			name:    "present predicate hit",
			fields:  map[string]any{"msg": Predicate{Kind: "present"}},
			payload: map[string]any{"msg": "anything"},
			want:    true,
		},
		{
			name:    "present predicate miss",
			fields:  map[string]any{"msg": Predicate{Kind: "present"}},
			payload: map[string]any{"level": float64(1)},
			want:    false,
		},
		{
			name:    "contains predicate hit",
			fields:  map[string]any{"msg": Predicate{Kind: "contains", Arg: "ell"}},
			payload: map[string]any{"msg": "hello"},
			want:    true,
		},
		{
			name:    "contains predicate on non-string",
			fields:  map[string]any{"msg": Predicate{Kind: "contains", Arg: "1"}},
			payload: map[string]any{"msg": float64(1)},
			want:    false,
		},
		{
			name:    "regex predicate hit",
			fields:  map[string]any{"msg": Predicate{Kind: "regex", Arg: "^h.*o$"}},
			payload: map[string]any{"msg": "hello"},
			want:    true,
		},
		{
			name:    "regex predicate miss",
			fields:  map[string]any{"msg": Predicate{Kind: "regex", Arg: "^x"}},
			payload: map[string]any{"msg": "hello"},
			want:    false,
		},
		{
			name:   "predicate nested in map",
			fields: map[string]any{"req": map[string]any{"msg": Predicate{Kind: "contains", Arg: "boo"}}},
			payload: map[string]any{"req": map[string]any{
				"msg": "boom", "other": "x",
			}},
			want: true,
		},
		{
			name:    "bool match",
			fields:  map[string]any{"read_only": true},
			payload: map[string]any{"read_only": true},
			want:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := NewMatcher(tc.fields)
			assert.Equal(t, tc.want, m.Match(tc.payload))
		})
	}
}

func TestNewMatcherEmptyIsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, NewMatcher(nil))
	assert.Nil(t, NewMatcher(map[string]any{}))
}

func TestMatcherExplain(t *testing.T) {
	t.Parallel()

	m := NewMatcher(map[string]any{
		"msg":   "hello world",
		"level": 2,
		"gone":  Predicate{Kind: "present"},
	})
	got := m.Explain(map[string]any{"msg": "bye", "level": float64(2)})
	assert.Contains(t, got, `field "msg": expected "hello world", got "bye"`)
	assert.Contains(t, got, `field "gone": expected present(), but field is absent`)
	assert.NotContains(t, got, `field "level"`)
}

func TestMatchTimeoutErrorMessage(t *testing.T) {
	t.Parallel()

	err := &MatchTimeoutError{
		Method:  "browser.Notifications/Notify",
		Matcher: NewMatcher(map[string]any{"msg": "hello"}),
		Observed: []Snapshot{
			{Method: "browser.Notifications/Notify", Request: map[string]any{"msg": "bye"}},
		},
	}
	msg := err.Error()
	assert.Contains(t, msg, "browser.Notifications/Notify")
	assert.Contains(t, msg, "1 unmatched call(s)")
	assert.Contains(t, msg, `expected "hello", got "bye"`)

	empty := &MatchTimeoutError{Method: "x.Y/Z", Matcher: nil}
	assert.Contains(t, empty.Error(), "no unmatched calls")
	assert.Contains(t, empty.Error(), "<any request>")
}

func TestMethodIgnored(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method string
		ignore []string
		want   bool
	}{
		{"config.Config/Get", []string{"config.Config/Get"}, true},
		{"config.Config/Get", []string{"config.Config/*"}, true},
		{"workspace.Files/Read", []string{"workspace.*/*"}, true},
		{"workspace.Files/Read", []string{"config.Config/*"}, false},
		{"workspace.Files/Read", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.method+"_"+strings.Join(tc.ignore, ","), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, methodIgnored(tc.method, tc.ignore))
		})
	}
}
