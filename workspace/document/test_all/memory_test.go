package test_all

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding/json"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
	workdoc "unstable.build/go-tui/workspace/document"
	"unstable.build/go-tui/workspace/test"
)

func TestMemoryWorkspaceScheme(t *testing.T) {
	t.Run("with folder", func(t *testing.T) {
		testWorkspaceSchemeSuite(t, func(t *testing.T) workspace.Scheme {
			workspaceURI, err := workspaceapi.ParseURI("inmemory:///tmp")
			require.NoError(t, err)
			svc := document.NewInMemoryService()
			scheme, err := workdoc.WorkspaceScheme[testStruct](workspaceURI, svc,
				json.Marshaler(), errMissingID)(config.NopConfig(), workspaceURI)
			require.NoError(t, err)
			return scheme
		})
	})
	t.Run("at root", func(t *testing.T) {
		testWorkspaceSchemeSuite(t, func(t *testing.T) workspace.Scheme {
			workspaceURI, err := workspaceapi.ParseURI("inmemory:///")
			require.NoError(t, err)
			svc := document.NewInMemoryService()
			scheme, err := workdoc.WorkspaceScheme[testStruct](workspaceURI, svc,
				json.Marshaler(), errMissingID)(config.NopConfig(), workspaceURI)
			require.NoError(t, err)
			return scheme
		})
	})
}

func TestMemoryWorkspaceSchemeTransientFailures(t *testing.T) {
	testWorkspaceSchemeSuite(t, func(t *testing.T) workspace.Scheme {
		workspaceURI, err := workspaceapi.ParseURI("inmemory:///")
		require.NoError(t, err)
		svc := &errService{root: document.NewInMemoryService()}
		s, err := workdoc.WorkspaceScheme[testStruct](workspaceURI, svc, json.Marshaler(),
			errMissingID)(config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return s
	})
}

func testWorkspaceSchemeSuite(
	t *testing.T,
	schemeFn func(t *testing.T) workspace.Scheme,
) {
	t.Run("Open", func(t *testing.T) {
		// there should not be any path manipulation with a document.Service
		// backed workspace.Scheme.
		const testPaths = false
		test.TestWorkspaceSchemeOpen(t, schemeFn,
			createTestFile, readContent, writeContent, testPaths)
	})
	t.Run("Remove", func(t *testing.T) {
		test.TestWorkspaceSchemeRemove(t, schemeFn, createTestFile)
	})
	t.Run("Rename", func(t *testing.T) {
		test.TestWorkspaceSchemeRename(t, schemeFn, createTestFile, readContent)
	})
	t.Run("Stat", func(t *testing.T) {
		test.TestWorkspaceSchemeStat(t, schemeFn, createTestFile)
	})
	t.Run("Lstat", func(t *testing.T) {
		test.TestWorkspaceSchemeLstat(t, schemeFn, createTestFile)
	})
	t.Run("Link", func(t *testing.T) {
		test.TestWorkspaceSchemeReadLink(t, schemeFn, createTestFile)
	})
	t.Run("ReadDir", func(t *testing.T) {
		test.TestWorkspaceSchemeReadDir(t, schemeFn, createTestFile)
	})
	t.Run("workspace.ListFiles integration", func(t *testing.T) {
		test.TestWorkspaceSchemeListFilesIntegration(t, schemeFn, createTestFile)
	})
	t.Run("workspace.Load integration", func(t *testing.T) {
		test.TestWorkspaceLoadIntegration(t, schemeFn,
			createTestFile, readContent, writeToBuffer)
	})
}

func createTestFile(t *testing.T, s workspace.Scheme, filename, content string) (workspaceapi.File, func()) {
	file, werr := s.Open(filename, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666)
	require.Nil(t, werr, werr.String())

	var doc testStruct
	doc.Content = content
	doc.Id = filename

	data, err := json.Marshaler().Marshal(doc)
	require.NoError(t, err)

	require.NoError(t, file.Truncate(0))

	_, err = file.Write(data)
	require.NoError(t, err)
	require.NoError(t, file.Sync())
	return file, func() {
		require.NoError(t, file.Close())
	}
}

func writeToBuffer(buf *cell.Buffer, data []byte) (int, error) {
	var doc testStruct

	err := json.Marshaler().Unmarshal([]byte(buf.String()), &doc)
	if err != nil {
		return 0, fmt.Errorf("Unmarshal: %v: %q", err, buf.String())
	}
	var builder strings.Builder
	builder.WriteString(doc.Content)
	builder.Write(data)
	doc.Content = builder.String()

	data, err = json.Marshaler().Marshal(doc)
	if err != nil {
		return 0, fmt.Errorf("Marshal: %v", err)
	}

	end := term.Coordinates{Y: buf.Rows(), X: buf.Columns(buf.Rows()-1) + 1}
	buf.Edit(term.Coordinates{}, end, string(data))
	return len(data), nil
}

func readContent(r io.Reader) ([]byte, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var temp testStruct
	err = json.Marshaler().Unmarshal(data, &temp)
	if err != nil {
		return nil, err
	}

	return []byte(temp.Content), nil
}

func writeContent(f workspaceapi.File, data []byte) (int, error) {
	allData, err := ioutil.ReadAll(f)
	if err != nil {
		return 0, err
	}
	var temp testStruct
	if len(allData) != 0 {
		err = json.Marshaler().Unmarshal(allData, &temp)
		if err != nil {
			return 0, err
		}
	} else {
		temp = temp.WithID(f.Name())
	}

	temp.Content = string(data)
	allData, err = json.Marshaler().Marshal(temp)
	if err != nil {
		return 0, err
	}
	if err := f.Truncate(0); err != nil {
		return 0, err
	}
	return f.Write(allData)
}

var errMissingID = errors.New("missing id")

type testStruct struct {
	Id        string
	UpdatedAt time.Time `firestore:"UpdatedAt,serverTimestamp"`
	Content   string
}

func (t testStruct) ID() string {
	return t.Id
}

func (t testStruct) WithID(id string) testStruct {
	t.Id = id
	return t
}

func (t testStruct) UpdatedTime() time.Time {
	return t.UpdatedAt
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
