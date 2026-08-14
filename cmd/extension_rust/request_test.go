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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errLSP fails every ExecuteRequest with a fixed error.
type errLSP struct {
	noopLSP
	err error
}

func (l *errLSP) Initialize(
	_ context.Context, _ semanticapi.InitializeParams,
) (semanticapi.InitializeResult, error) {
	return semanticapi.InitializeResult{}, nil
}

func (l *errLSP) ExecuteRequest(
	_ context.Context, _ semanticapi.ExecuteRequestParams,
) (json.RawMessage, error) {
	return nil, l.err
}

// Extension requests travel over gRPC: a server-side failure arrives as
// a status error whose rendering buries the actual rust-analyzer
// message under "rpc error: code = Unknown desc = ...". The user-facing
// error must carry only the server's message.
func TestExecRequestStripsGRPCStatusNoise(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "grpc status error",
			err: status.Error(codes.Unknown,
				"request handler panicked: index out of bounds: the len is 1 but the index is 18"),
			want: "rust-analyzer/viewMir: request handler panicked: " +
				"index out of bounds: the len is 1 but the index is 18",
		},
		{
			name: "plain error",
			err:  errors.New("connection lost"),
			want: "rust-analyzer/viewMir: connection lost",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := execRequest[string](
				context.Background(), &errLSP{err: tc.err}, "rust-analyzer/viewMir", nil)
			require.Error(t, err)
			assert.Equal(t, tc.want, err.Error())
			assert.NotContains(t, err.Error(), "rpc error")
		})
	}
}
