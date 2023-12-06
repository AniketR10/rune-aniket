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
	"unstable.build/go-tui/proto"
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
	broker := proto.NewUnixGRPCBroker("")
	defer broker.Close()

	edMock := texttest.NewMockEditor(ctrl)
	resources := extension.EditorResources(edMock)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = proto.ContextWithWaitGroup(ctx, new(sync.WaitGroup))

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
		createResource func(extension.Grant, proto.MuxBroker) (interface{}, error)
		expect         func(*texttest.MockEditorMockRecorder) *gomock.Call
		method         func(ifc interface{}) error
	}{
		{extension.PermissionEditor, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(ed *texttest.MockEditorMockRecorder) *gomock.Call {
			return ed.SubscribeEvents(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			h := text.FuncEventHandler(func(context.Context, textapi.Event) bool { return false })
			ev := []textapi.EventType{textapi.EventTypeFlush}
			return ifc.(textapi.Editor).SubscribeEvents(ev, h)
		}},
		{extension.PermissionEditor, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(ed *texttest.MockEditorMockRecorder) *gomock.Call {
			ed.Editor(gomock.Any()).Return(th, nil).AnyTimes()
			return ed.SetCursor(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(textapi.Editor).SetCursor(th, term.Coordinates{})
		}},
		{extension.PermissionEditor, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
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
