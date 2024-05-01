package test_all

import (
	"context"
	"os"
	"testing"

	"github.com/unstablebuild/blue/document/firestore"
	"github.com/unstablebuild/blue/encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
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
	teardown := runFirestoreOrSkip(t)
	defer teardown()

	testWorkspaceSchemeSuite(t, func(t *testing.T) schemeapi.Scheme {
		testProjectID := uuid.New().String()
		collection := uuid.New().String()

		ctx := context.Background()

		workspaceURI, err := workspaceapi.ParseURI("firestore:///tmp")
		require.NoError(t, err)
		svc, err := firestore.New(testProjectID, collection, "")
		require.NoError(t, err)
		scheme, err := workdoc.WorkspaceScheme[testStruct](workspaceURI, svc,
			json.Marshaler(), errMissingID)(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return scheme
	})

	t.Run("cleans absolute paths", func(t *testing.T) {
		testProjectID := uuid.New().String()
		collection := uuid.New().String()

		ctx := context.Background()

		workspaceURI, err := workspaceapi.ParseURI("firestore:///tmp")
		require.NoError(t, err)
		svc, err := firestore.New(testProjectID, collection, "")
		require.NoError(t, err)
		scheme, err := workdoc.WorkspaceScheme[testStruct](workspaceURI, svc,
			json.Marshaler(), errMissingID)(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		f, werr := scheme.Open("/.something.swp", os.O_CREATE, 0)
		require.Nil(t, werr)
		require.NoError(t, f.Close())
	})
}
