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

package lspcmd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func TestNormalizeMethodName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		// Pointer receiver methods — the core case.
		{
			name: "simple pointer receiver",
			in:   "(*Greeter).Greet",
			want: "Greeter.Greet",
		},
		{
			name: "pointer receiver with short name",
			in:   "(*T).M",
			want: "T.M",
		},
		{
			name: "pointer receiver with long type name",
			in:   "(*definitionHandler).HandleCommand",
			want: "definitionHandler.HandleCommand",
		},
		{
			name: "pointer receiver with unexported type",
			in:   "(*routerHandler).Handle",
			want: "routerHandler.Handle",
		},
		{
			name: "pointer receiver with underscore type",
			in:   "(*my_type).do_thing",
			want: "my_type.do_thing",
		},
		{
			name: "pointer receiver with numeric suffix",
			in:   "(*Handler2).ServeHTTP",
			want: "Handler2.ServeHTTP",
		},

		// Value receiver methods.
		{
			name: "value receiver",
			in:   "(Counter).Count",
			want: "Counter.Count",
		},
		{
			name: "value receiver short",
			in:   "(T).String",
			want: "T.String",
		},
		{
			name: "value receiver with unexported type",
			in:   "(myStruct).value",
			want: "myStruct.value",
		},

		// Non-method symbols — should pass through unchanged.
		{
			name: "plain function",
			in:   "Add",
			want: "Add",
		},
		{
			name: "plain type",
			in:   "Greeter",
			want: "Greeter",
		},
		{
			name: "constant",
			in:   "MaxSize",
			want: "MaxSize",
		},
		{
			name: "qualified symbol",
			in:   "mylib.MyType",
			want: "mylib.MyType",
		},
		{
			name: "value receiver method already normalized",
			in:   "Counter.Count",
			want: "Counter.Count",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},

		// Edge cases — malformed but shouldn't panic.
		{
			name: "open paren star without close",
			in:   "(*Foo",
			want: "(*Foo",
		},
		{
			name: "only prefix",
			in:   "(*",
			want: "(*",
		},
		{
			name: "paren star with close but no dot",
			in:   "(*Foo)",
			want: "(*Foo)",
		},
		{
			name: "nested parens are not mangled",
			in:   "func(*int)",
			want: "func(*int)",
		},
		{
			name: "open paren without close",
			in:   "(Foo",
			want: "(Foo",
		},
		{
			name: "paren with close but no dot",
			in:   "(Foo)",
			want: "(Foo)",
		},
		{
			name: "star without paren prefix",
			in:   "*Foo.Bar",
			want: "*Foo.Bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, normalizeMethodName(tt.in))
		})
	}
}

// TestCompleteReferencedSymbol verifies that completeReferencedSymbol streams
// the names produced by Parser.ListReferencedSymbols and deduplicates them,
// preserving first-seen order. The per-language query logic that produces the
// underlying names is exercised by the symbolresolve engine tests.
func TestCompleteReferencedSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{
			name:  "passes names through",
			names: []string{"context.Context", "fmt.Errorf", "iterator.Map"},
			want:  []string{"context.Context", "fmt.Errorf", "iterator.Map"},
		},
		{
			name:  "deduplicates repeated names keeping first-seen order",
			names: []string{"fmt.Errorf", "fmt.Println", "fmt.Errorf", "io.Reader", "fmt.Println"},
			want:  []string{"fmt.Errorf", "fmt.Println", "io.Reader"},
		},
		{
			name:  "empty stream",
			names: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			names := tt.names
			parser := &mockParser{
				listReferencedFn: func() (iterator.Iterator[string], error) {
					return iterator.FromSlice(names), nil
				},
			}
			iter, err := completeReferencedSymbol(context.Background(), parser)
			require.NoError(t, err)
			got := collectIter(t, iter)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompleteReferencedSymbolPropagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("no focused parser")
	parser := &mockParser{
		listReferencedFn: func() (iterator.Iterator[string], error) {
			return nil, wantErr
		},
	}
	_, err := completeReferencedSymbol(context.Background(), parser)
	assert.ErrorIs(t, err, wantErr)
}
