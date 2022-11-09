package document

import (
	"context"
	"errors"
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/ernestrc/blue/retry"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	workspacedoc "unstable.build/go-tui/storage/workspace"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/test"
)

func TestServiceWorkspaceScheme(t *testing.T) {
	t.Run("with folder", func(t *testing.T) {
		test.TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
			workspaceURI, err := workspace.ParseURI("inmemory:///tmp")
			require.NoError(t, err)
			svc := document.NewInMemoryService()
			scheme, err := WorkspaceScheme(workspaceURI, svc)(config.NopConfig(), workspaceURI)
			require.NoError(t, err)
			return scheme
		})
	})
	t.Run("at root", func(t *testing.T) {
		test.TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
			workspaceURI, err := workspace.ParseURI("inmemory:///")
			require.NoError(t, err)
			svc := document.NewInMemoryService()
			scheme, err := WorkspaceScheme(workspaceURI, svc)(config.NopConfig(), workspaceURI)
			require.NoError(t, err)
			return scheme
		})
	})
}

type errService struct {
	root              document.Service
	nextFailureCreate bool
	nextFailureSet    bool
	nextFailureUpdate bool
	nextFailureGet    bool
	nextFailureDelete bool
	nextFailureList   bool
}

func (s *errService) Create(ctx context.Context, ID string, doc interface{}) error {
	s.nextFailureCreate = !s.nextFailureCreate
	if s.nextFailureCreate {
		return errors.New("Create transient error")
	}
	return s.root.Create(ctx, ID, doc)
}

func (s *errService) Set(ctx context.Context, ID string, doc interface{}) error {
	s.nextFailureSet = !s.nextFailureSet
	if s.nextFailureSet {
		return errors.New("Set transient error")
	}
	return s.root.Set(ctx, ID, doc)
}

func (s *errService) Update(ctx context.Context, ID string,
	updates []document.Update, precond ...document.Precondition) error {
	s.nextFailureUpdate = !s.nextFailureUpdate
	if s.nextFailureUpdate {
		return errors.New("Update transient error")
	}
	return s.root.Update(ctx, ID, updates, precond...)
}

func (s *errService) Get(ctx context.Context, ID string, doc interface{}) error {
	s.nextFailureGet = !s.nextFailureGet
	if s.nextFailureGet {
		return errors.New("Get transient error")
	}
	return s.root.Get(ctx, ID, doc)
}

func (s *errService) Delete(ctx context.Context, ID string) error {
	s.nextFailureDelete = !s.nextFailureDelete
	if s.nextFailureDelete {
		return errors.New("Delete transient error")
	}
	return s.root.Delete(ctx, ID)
}

func (s *errService) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	s.nextFailureList = !s.nextFailureList
	if s.nextFailureList {
		return nil, errors.New("List transient error")
	}
	return s.root.List(ctx, filters)
}

func (s *errService) Close() error {
	return s.root.Close()
}

func TestServiceWorkspaceSchemeTransientFailures(t *testing.T) {
	test.TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
		workspaceURI, err := workspace.ParseURI("inmemory:///")
		require.NoError(t, err)
		svc := &errService{root: document.NewInMemoryService()}
		s, err := WorkspaceScheme(workspaceURI, svc)(config.NopConfig(), workspaceURI)
		s.(*scheme).retryRealFailure = retry.LimitStrategy(2)
		s.(*scheme).retryInconsistency = retry.LimitStrategy(2)
		require.NoError(t, err)
		return s
	})
}

func TestServiceWorkspaceSchemeBackedByWorkspaceSchemeService(t *testing.T) {
	test.TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
		workspaceURI, err := workspace.ParseURI("memory:///")
		require.NoError(t, err)

		scheme, err := workspace.NewMemoryScheme(config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		svc, err := workspacedoc.NewWorkspaceService(scheme, json.Marshaler())
		require.NoError(t, err)

		s, err := WorkspaceScheme(workspaceURI, svc)(config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return s
	})
}

// TODO test hard failure scenarious no leaking data or corrupted state
