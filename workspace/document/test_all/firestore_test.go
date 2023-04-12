package test_all

import (
	"context"
	"testing"

	"github.com/ernestrc/blue/document/firestore"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/workspace"
	workdoc "unstable.build/go-tui/workspace/document"
)

func runFirestoreOrSkip(t *testing.T) func() {
	teardown, err := firestore.RunFirestoreEmulator()
	if err != nil {
		t.Logf("error with firestore emulator, skipping test: %s", err)
		t.SkipNow()
		return func() {}
	}
	return func() {
		err := teardown()
		if err != nil {
			t.Logf("error closing firestore emulator: %s", err)
		}
	}
}

func TestFirestoreWorkspaceScheme(t *testing.T) {
	testWorkspaceSchemeSuite(t, func(t *testing.T) workspace.Scheme {
		testProjectID := uuid.New().String()
		collection := uuid.New().String()

		teardown := runFirestoreOrSkip(t)
		defer teardown()

		ctx := context.Background()

		workspaceURI, err := workspaceapi.ParseURI("inmemory:///tmp")
		require.NoError(t, err)
		svc, err := firestore.New(testProjectID, collection, "")
		require.NoError(t, err)
		scheme, err := workdoc.WorkspaceScheme[testStruct](workspaceURI, svc,
			json.Marshaler(), errMissingID)(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return scheme
	})
}
