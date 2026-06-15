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

package llmrpc

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi/llmrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeCompletionStream replays a fixed sequence of chunks to recvCompletionRequest.
type fakeCompletionStream struct {
	grpc.BidiStreamingServer[llmrpc.CreateCompletionRequestChunk, llmrpc.CreateCompletionResponseChunk]
	chunks []*llmrpc.CreateCompletionRequestChunk
	idx    int
}

func (f *fakeCompletionStream) Recv() (*llmrpc.CreateCompletionRequestChunk, error) {
	if f.idx >= len(f.chunks) {
		return nil, io.EOF
	}
	c := f.chunks[f.idx]
	f.idx++
	return c, nil
}

func headerChunk(h *llmrpc.CompletionHeader) *llmrpc.CreateCompletionRequestChunk {
	return &llmrpc.CreateCompletionRequestChunk{
		Payload: &llmrpc.CreateCompletionRequestChunk_Header{Header: h},
	}
}

func messageChunk(m *llmrpc.Message) *llmrpc.CreateCompletionRequestChunk {
	return &llmrpc.CreateCompletionRequestChunk{
		Payload: &llmrpc.CreateCompletionRequestChunk_Message{Message: m},
	}
}

func TestRecvCompletionRequest(t *testing.T) {
	tests := []struct {
		name     string
		chunks   []*llmrpc.CreateCompletionRequestChunk
		wantMsgs int
		wantCode codes.Code
	}{
		{
			name:     "header only",
			chunks:   []*llmrpc.CreateCompletionRequestChunk{headerChunk(&llmrpc.CompletionHeader{})},
			wantMsgs: 0,
		},
		{
			name: "header then messages preserve order",
			chunks: []*llmrpc.CreateCompletionRequestChunk{
				headerChunk(&llmrpc.CompletionHeader{}),
				messageChunk(&llmrpc.Message{Content: "a"}),
				messageChunk(&llmrpc.Message{Content: "b"}),
			},
			wantMsgs: 2,
		},
		{
			name:     "missing header",
			chunks:   nil,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "message before header",
			chunks: []*llmrpc.CreateCompletionRequestChunk{
				messageChunk(&llmrpc.Message{Content: "a"}),
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "duplicate header",
			chunks: []*llmrpc.CreateCompletionRequestChunk{
				headerChunk(&llmrpc.CompletionHeader{}),
				headerChunk(&llmrpc.CompletionHeader{}),
			},
			wantCode: codes.InvalidArgument,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header, msgs, err := recvCompletionRequest(&fakeCompletionStream{chunks: tc.chunks})
			if tc.wantCode != codes.OK {
				require.Error(t, err)
				assert.Equal(t, tc.wantCode, status.Code(err))
				return
			}
			require.NoError(t, err)
			require.NotNil(t, header)
			assert.Len(t, msgs, tc.wantMsgs)
			for i := range msgs {
				assert.Equal(t, tc.chunks[i+1].GetMessage().GetContent(), msgs[i].GetContent())
			}
		})
	}
}
