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
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// Predicate is a non-exact field matcher.
type Predicate struct {
	Kind string // "present" | "contains" | "regex"
	Arg  string
}

func (p Predicate) match(v any, ok bool) bool {
	switch p.Kind {
	case "present":
		return ok
	case "contains":
		if !ok {
			return false
		}
		s, isStr := v.(string)
		return isStr && strings.Contains(s, p.Arg)
	case "regex":
		if !ok {
			return false
		}
		s, isStr := v.(string)
		if !isStr {
			return false
		}
		matched, err := regexp.MatchString(p.Arg, s)
		return err == nil && matched
	default:
		return false
	}
}

func (p Predicate) String() string {
	if p.Kind == "present" {
		return "present()"
	}
	return fmt.Sprintf("%s(%q)", p.Kind, p.Arg)
}

// Matcher matches a request payload against expected field values.
// Keys may use dots to address nested fields ("range.start.line").
// Values are compared exactly (maps match as subsets, recursively)
// unless the value is a Predicate.
type Matcher struct {
	Fields map[string]any
}

// NewMatcher returns a Matcher over the given field expectations, or
// nil when fields is empty so callers can treat "no matcher" and "no
// fields" uniformly.
func NewMatcher(fields map[string]any) *Matcher {
	if len(fields) == 0 {
		return nil
	}
	return &Matcher{Fields: fields}
}

// Match reports whether payload satisfies every expected field.
func (m *Matcher) Match(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	for key, want := range m.Fields {
		got, ok := lookupPath(payload, key)
		if !valueMatches(want, got, ok) {
			return false
		}
	}
	return true
}

// Explain returns a human-readable description of which expected
// fields payload fails to satisfy, one line per mismatch.
func (m *Matcher) Explain(payload map[string]any) string {
	keys := make([]string, 0, len(m.Fields))
	for k := range m.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		want := m.Fields[key]
		got, ok := lookupPath(payload, key)
		if valueMatches(want, got, ok) {
			continue
		}
		if !ok {
			fmt.Fprintf(&b, "    field %q: expected %s, but field is absent\n",
				key, formatValue(want))
			continue
		}
		fmt.Fprintf(&b, "    field %q: expected %s, got %s\n",
			key, formatValue(want), formatValue(got))
	}
	return b.String()
}

// String renders the matcher for failure reports.
func (m *Matcher) String() string {
	if m == nil {
		return "<any request>"
	}
	keys := make([]string, 0, len(m.Fields))
	for k := range m.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, formatValue(m.Fields[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func valueMatches(want, got any, ok bool) bool {
	if p, isPred := want.(Predicate); isPred {
		return p.match(got, ok)
	}
	if !ok {
		return false
	}
	return deepMatch(want, got)
}

// deepMatch compares an expected value against an observed one.
// Numbers are normalized (payloads come from JSON as float64), maps
// match as subsets so specs only state the fields they care about.
func deepMatch(want, got any) bool {
	if p, isPred := want.(Predicate); isPred {
		return p.match(got, true)
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, wv := range w {
			gv, ok := g[k]
			if p, isPred := wv.(Predicate); isPred {
				if !p.match(gv, ok) {
					return false
				}
				continue
			}
			if !ok || !deepMatch(wv, gv) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !deepMatch(w[i], g[i]) {
				return false
			}
		}
		return true
	default:
		if wn, ok := toFloat(want); ok {
			gn, ok := toFloat(got)
			return ok && wn == gn
		}
		return reflect.DeepEqual(want, got)
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func lookupPath(payload map[string]any, key string) (any, bool) {
	parts := strings.Split(key, ".")
	var cur any = payload
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func formatValue(v any) string {
	if p, ok := v.(Predicate); ok {
		return p.String()
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

func methodIgnored(method string, ignore []string) bool {
	for _, pat := range ignore {
		if pat == method {
			return true
		}
		if ok, err := path.Match(pat, method); err == nil && ok {
			return true
		}
	}
	return false
}

// MatchTimeoutError reports an expectation that was not satisfied in
// time, including what was observed for the same method so failures
// are actionable.
type MatchTimeoutError struct {
	Method   string
	Matcher  *Matcher
	Observed []Snapshot
}

func (e *MatchTimeoutError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "expected rpc %s matching %s was not observed",
		e.Method, e.Matcher.String())
	if len(e.Observed) == 0 {
		b.WriteString("; no unmatched calls to this method were recorded")
		return b.String()
	}
	fmt.Fprintf(&b, "; %d unmatched call(s) to this method:", len(e.Observed))
	for i, obs := range e.Observed {
		fmt.Fprintf(&b, "\n  call %d: request=%s", i+1, formatValue(obs.Request))
		if e.Matcher != nil && obs.Request != nil {
			if diff := e.Matcher.Explain(obs.Request); diff != "" {
				b.WriteString("\n" + strings.TrimRight(diff, "\n"))
			}
		}
	}
	return b.String()
}

func jsonToMap(data []byte) map[string]any {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{"_unmarshal_error": err.Error()}
	}
	return m
}
