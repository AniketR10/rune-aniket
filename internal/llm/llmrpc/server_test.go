// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
