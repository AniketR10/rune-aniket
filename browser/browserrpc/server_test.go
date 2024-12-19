// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package browserrpc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser/browsertest"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	termrpc "unstable.build/go-tui/term/termrpc"
)

const asyncResultsSleepDuration = 300 * time.Millisecond

func newServerWithNoBroker(ctrl *gomock.Controller) (
	*Server, *browsertest.MockBrowser,
) {
	mock := browsertest.NewMockBrowser(ctrl)
	s := NewServer(nil, mock, new(sync.Mutex))
	s.SetSyncMode()
	return s, mock
}

func newTestServer(ctrl *gomock.Controller, mu *sync.Mutex) (
	*Server, *browsertest.MockBrowser, *rpc.MockMuxBroker,
) {
	mockBrowser := browsertest.NewMockBrowser(ctrl)
	mockBroker := rpc.NewMockMuxBroker(ctrl)
	s := NewServer(mockBroker, mockBrowser, mu)
	s.SetSyncMode()

	return s, mockBrowser, mockBroker
}

func TestServerNotify(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates Notify to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().Notify(gomock.Eq(notifications.LevelSuccess), gomock.Eq("blah")).Return(nil)

		req := NotifyRequest{Level: uint32(notifications.LevelSuccess), Msg: "blah"}
		res, err := s.Notify(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("bubbles up Notify Browser error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock := newServerWithNoBroker(ctrl)

		mock.EXPECT().Notify(gomock.Any(), gomock.Any()).Return(errors.New("oopsie daisy"))

		_, err := s.Notify(ctx, new(NotifyRequest))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie")
	})
}

func TestServerOpen(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates Open to underlying Browser", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, _ := newTestServer(ctrl, new(sync.Mutex))
		uri, err := workspaceapi.ParseURI("file:///tmp/coronavirus.sql")
		require.NoError(t, err)

		h := browsertest.NewTestHandler()
		mock.EXPECT().Open(gomock.Eq(uri)).Return(h, nil)

		req := OpenResourceRequest{Resource: "file:///tmp/coronavirus.sql"}
		res, err := s.Open(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("bubbles up Open Browser error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		s, mock, _ := newTestServer(ctrl, new(sync.Mutex))

		mock.EXPECT().Open(gomock.Any()).Return(nil, errors.New("oopsie daisy"))

		req := OpenResourceRequest{Resource: "file:///a"}
		_, err := s.Open(ctx, &req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "oopsie")
	})
}

func TestServerPublish(t *testing.T) {
	ctx := context.Background()

	t.Run("handles interrupt event by calling interrupt handler", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := PublishRequest{Ev: &termrpc.Event{Type: termrpc.Event_TypeInterrupt}}

		mock.EXPECT().PublishEvent(gomock.Eq(term.Event{Type: term.EventInterrupt})).Times(1)

		res, err := s.Publish(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("handles EventNone event by calling interrupt handler", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := PublishRequest{Ev: &termrpc.Event{Type: termrpc.Event_TypeNone}}

		mock.EXPECT().PublishEvent(gomock.Eq(term.Event{Type: term.EventNone})).Times(1)

		res, err := s.Publish(ctx, &req)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("handles interrupt handler error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := PublishRequest{Ev: &termrpc.Event{Type: termrpc.Event_TypeInterrupt}}

		mock.EXPECT().PublishEvent(gomock.Any()).Return(errors.New("uRock"))

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("handles event none handler error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		var mu sync.Mutex
		s, mock, _ := newTestServer(ctrl, &mu)
		req := PublishRequest{Ev: &termrpc.Event{Type: termrpc.Event_TypeNone}}

		mock.EXPECT().PublishEvent(gomock.Any()).Return(errors.New("uRock"))

		res, err := s.Publish(ctx, &req)
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestServerSetContent(t *testing.T) {
	/* tested via ex integration tests */
}
