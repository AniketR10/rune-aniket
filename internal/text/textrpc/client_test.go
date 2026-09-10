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

package textrpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
	gomock "go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"unstable.build/rune/internal/cell"
	"unstable.build/rune/internal/rpc/rpctest"
	"unstable.build/rune/internal/text"
)

func TestClientEdit(t *testing.T) {
	bufContent1 := "The Maytals"

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cc, c := newTestClient(ctrl)
		uri, err := workspaceapi.ParseURI("file:///tmp/hello")
		require.NoError(t, err)

		buf := cell.NewBuffer()
		buf.WriteString(bufContent1)

		expectClientEdit(t, cc, bufContent1, uri)

		h, err := c.Edit(uri, buf, false, false)
		require.NoError(t, err)
		require.NotNil(t, h)
	})

	t.Run("handles underlying client error ", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		cc, c := newTestClient(ctrl)

		cc.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/text.Editor/Edit"),
				gomock.Any(),
				gomock.Any(), gomock.Any()).
			Times(1).
			Return(errors.New("Would be a change of plan"))

		h, err := c.Edit(workspaceapi.URI{}, cell.NewBuffer(), false, false)
		require.Error(t, err)
		require.Nil(t, h)
	})
}

func TestSetLocationListRequest(t *testing.T) {
	uri, err := workspaceapi.ParseURI("test:///")
	require.NoError(t, err)
	t.Run("non-nil zero slice", func(t *testing.T) {
		l := text.LocationSlice([]textapi.Location{})
		expected := textrpc.SetLocationListRequest{
			ResourceName: NewURI(uri),
			ListId:       locID,
			Locations:    nil,
			Priority:     2,
		}
		assert.Equal(t, expected, makeLocationListRequest(uri, textapi.LocationPriorityError, locID, l))
	})

	t.Run("nil zero slice", func(t *testing.T) {
		l := text.LocationSlice(nil)
		expected := textrpc.SetLocationListRequest{
			Priority:     2,
			ResourceName: NewURI(uri),
			ListId:       locID,
			Locations:    nil,
		}
		assert.Equal(t, expected, makeLocationListRequest(uri, textapi.LocationPriorityError, locID, l))
	})
	t.Run("non-zero slice", func(t *testing.T) {
		l := text.LocationSlice([]textapi.Location{
			loc1,
			loc2,
			loc3,
		})
		expected := textrpc.SetLocationListRequest{
			ResourceName: NewURI(uri),
			ListId:       locID,
			Priority:     2,
			Locations: []*textrpc.SetLocationListRequest_Location{
				{
					From: &termrpc.Coordinates{},
					To:   &termrpc.Coordinates{X: 1, Y: 3},
					Attr: &termrpc.Attributes{Attrs: uint32(term.AttrBold)},
				},
				{
					To:   &termrpc.Coordinates{},
					From: &termrpc.Coordinates{X: 1, Y: 3},
					Attr: &termrpc.Attributes{
						Foreground: uint32(term.ColorBlack),
						Background: uint32(term.ColorGreen),
					},
					Msg: "wsb: hold BBBY",
				},
				{
					From: &termrpc.Coordinates{},
					To:   &termrpc.Coordinates{},
					Attr: &termrpc.Attributes{},
				},
			},
		}

		assert.Equal(t, expected, makeLocationListRequest(uri, textapi.LocationPriorityError, locID, l))
	})
	t.Run("with message", func(t *testing.T) {
		myMsg := "wsb: HOLD GME"
		l := text.LocationSlice([]textapi.Location{
			{
				To:      term.Coordinates{X: 1, Y: 3},
				Attr:    term.Attributes{Attrs: term.AttrBold},
				Message: myMsg,
			},
		})
		expected := textrpc.SetLocationListRequest{
			Priority:     2,
			ResourceName: NewURI(uri),
			ListId:       locID,
			Locations: []*textrpc.SetLocationListRequest_Location{
				{
					From: &termrpc.Coordinates{},
					To:   &termrpc.Coordinates{X: 1, Y: 3},
					Attr: &termrpc.Attributes{Attrs: uint32(term.AttrBold)},
					Msg:  myMsg,
				},
			},
		}

		assert.Equal(t, expected, makeLocationListRequest(uri, textapi.LocationPriorityError, locID, l))
	})
}

func benchmarkSetLocationListRequest(b *testing.B, n int) {
	l := make([]textapi.Location, n)
	for i := range n {
		l[i] = textapi.Location{
			From: term.Coordinates{X: i, Y: n},
			To:   term.Coordinates{X: n, Y: n},
			Attr: term.Attributes{Attrs: term.AttrBold},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ll := text.LocationSlice(l)
		_ = makeLocationListRequest(uri, textapi.LocationPriorityError, locID, ll)
	}
}

func BenchmarkSetLocationListRequest10(b *testing.B) {
	benchmarkSetLocationListRequest(b, 10)
}

func BenchmarkSetLocationListRequest100(b *testing.B) {
	benchmarkSetLocationListRequest(b, 100)
}

func BenchmarkSetLocationListRequest1000(b *testing.B) {
	benchmarkSetLocationListRequest(b, 1000)
}

func BenchmarkSetLocationListRequest10000(b *testing.B) {
	benchmarkSetLocationListRequest(b, 10000)
}

var (
	loc1 = textapi.Location{
		To:   term.Coordinates{X: 1, Y: 3},
		Attr: term.Attributes{Attrs: term.AttrBold},
	}
	loc2 = textapi.Location{
		From:    term.Coordinates{X: 1, Y: 3},
		Attr:    term.Attributes{Fg: term.ColorBlack, Bg: term.ColorGreen},
		Message: "wsb: hold BBBY",
	}
	loc3 = textapi.Location{}
)

func newTestClient(ctrl *gomock.Controller) (
	*rpctest.MockClientConnInterface, *Client,
) {
	cc := rpctest.NewMockClientConnInterface(ctrl)
	c := NewClient(context.Background(), cc)
	return cc, c
}

func expectClientEdit(
	t *testing.T, mockCC *rpctest.MockClientConnInterface,
	expectedContent string, uri workspaceapi.URI,
) {
	mockCC.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/text.Editor/Edit"),
			gomock.Any(),
			gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, method string, args any,
			reply any, opts ...grpc.CallOption) error {
			editReq, ok := args.(*textrpc.EditRequest)
			require.True(t, ok)

			buf := EditRequestToBuffer(editReq)
			assert.Equal(t, expectedContent, buf.String())
			assert.Equal(t, uri.String(), editReq.ResourceName.GetUri())

			_, ok = reply.(*textrpc.EditResponse)
			assert.True(t, ok)
			return nil
		}).
		Times(1)
}

func makeLocationListRequest(
	uri workspaceapi.URI, priority textapi.LocationPriority,
	listID string, l textapi.LocationList,
) textrpc.SetLocationListRequest {
	req := textrpc.SetLocationListRequest{
		ResourceName: NewURI(uri),
		ListId:       listID,
		Priority:     uint32(priority),
	}

	for loc, ok := l.Current(); ok; loc, ok = l.Next() {
		var from, to termrpc.Coordinates
		var attr termrpc.Attributes
		from.FromModel(loc.From)
		to.FromModel(loc.To)
		attr.FromModel(loc.Attr)
		req.Locations = append(req.Locations, &textrpc.SetLocationListRequest_Location{
			From: &from,
			To:   &to,
			Attr: &attr,
			Msg:  loc.Message,
			Icon: loc.Icon,
		})
	}
	return req // nolint:govet
}
