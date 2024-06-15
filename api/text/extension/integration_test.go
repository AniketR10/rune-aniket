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
package extension

import (
	"context"
	_ "net/http/pprof"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	textpb "unstable.build/go-tui/text/rpc"
	texttest "unstable.build/go-tui/text/test"
)

func assertClientMethodNoError(
	t *testing.T, resource interface{},
	wg *sync.WaitGroup, mu sync.Locker,
	method func(ifc interface{}) error,
) {
	defer wg.Done()
	mu.Lock()
	defer mu.Unlock()
	err := method(resource)
	require.NoError(t, err)
}

func TestIntegrationRace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	term.DisableInterruptForTesting()

	uri, err := workspaceapi.ParseURI("file:///tmp/test")
	require.NoError(t, err)

	th := textpb.Token{URI: uri}
	broker := rpc.NewUnixGRPCBroker("", "", "")
	defer broker.Close()

	edMock := texttest.NewMockEditor(ctrl)
	resources := extension.EditorResources(edMock)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = rpc.ContextWithWaitGroup(ctx, new(sync.WaitGroup))

	grantor := extension.GrantAll(resources)
	srv, err := broker.NewChannel()
	require.NoError(t, err)
	grantID := extension.Grant{Token: srv.Addr().String(), Context: ctx}
	perms := map[extension.Permission]extension.Grant{
		extension.PermissionEditor: grantID,
	}

	for perm := range perms {
		resources[perm].Register("caliu-extensions-ltd", grantor,
			srv.Registrar(), broker, new(sync.Mutex))
	}

	go srv.Serve(ctx)

	tsuite := []struct {
		perm           extension.Permission
		createResource func(extension.Grant, rpc.MuxBroker) (interface{}, error)
		expect         func(*texttest.MockEditorMockRecorder) *gomock.Call
		method         func(ifc interface{}) error
	}{
		{extension.PermissionEditor, func(token extension.Grant, broker rpc.MuxBroker) (interface{}, error) {
			return Editor(context.Background(), token, broker)
		}, func(ed *texttest.MockEditorMockRecorder) *gomock.Call {
			return ed.SubscribeEvents(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			h := text.FuncEventHandler(func(context.Context, textapi.Event) bool { return false })
			ev := []textapi.EventType{textapi.EventTypeFlush}
			return ifc.(textapi.Editor).SubscribeEvents(ev, h)
		}},
		{extension.PermissionEditor, func(token extension.Grant, broker rpc.MuxBroker) (interface{}, error) {
			return Editor(context.Background(), token, broker)
		}, func(ed *texttest.MockEditorMockRecorder) *gomock.Call {
			return ed.SubscribeCommand(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			h := textapi.FuncCommandHandler(nil, nil)
			return ifc.(textapi.Editor).SubscribeCommand(textapi.CommandManual{}, h)
		}},
		{extension.PermissionEditor, func(token extension.Grant, broker rpc.MuxBroker) (interface{}, error) {
			return Editor(context.Background(), token, broker)
		}, func(ed *texttest.MockEditorMockRecorder) *gomock.Call {
			ed.Editor(gomock.Any()).Return(th, nil).AnyTimes()
			return ed.SetCursor(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(textapi.Editor).SetCursor(th, term.Coordinates{})
		}},
		{extension.PermissionEditor, func(token extension.Grant, broker rpc.MuxBroker) (interface{}, error) {
			return Editor(context.Background(), token, broker)
		}, func(ed *texttest.MockEditorMockRecorder) *gomock.Call {
			ed.Editor(gomock.Any()).Return(th, nil).AnyTimes()
			return ed.Cursor(gomock.Any()).Return(term.Coordinates{}, nil)
		}, func(ifc interface{}) error {
			_, err = ifc.(textapi.Editor).Cursor(th)
			return err
		}},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, tcase := range tsuite {
		brokerID := perms[tcase.perm]
		res1, err := tcase.createResource(brokerID, broker)
		require.NoError(t, err)

		res2, err := tcase.createResource(brokerID, broker)
		require.NoError(t, err)

		edMock.EXPECT().UnsubscribeEvents(gomock.Any()).AnyTimes()

		n := 5
		if tcase.expect != nil {
			tcase.expect(edMock.EXPECT()).Times(n * 2)
		}
		for i := 0; i < n; i++ {
			wg.Add(2)
			go assertClientMethodNoError(t, res1, &wg, &mu, tcase.method)
			go assertClientMethodNoError(t, res2, &wg, &mu, tcase.method)
		}

	}
	wg.Wait()
}
