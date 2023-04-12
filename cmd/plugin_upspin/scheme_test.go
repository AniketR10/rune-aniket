package main

import (
	"testing"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/stretchr/testify/require"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace/test"
	"upspin.io/test/testenv"
	"upspin.io/upspin"
)

type bindClose struct {
	schemeapi.Scheme
	*testenv.Env
}

func (s bindClose) Close() (ret error) {
	if err := s.Env.Exit(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := s.Scheme.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return
}

func TestScheme(t *testing.T) {
	t.Run("plain packing", func(t *testing.T) {
		t.Run("root of path", func(t *testing.T) {
			test.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
				setup := &testenv.Setup{
					OwnerName: upspin.UserName("user1@domain.com"),
					Kind:      "inprocess",
					Packing:   upspin.PlainPack,
				}
				env, err := testenv.New(setup)
				require.NoError(t, err)

				_, err = env.Client.MakeDirectory("user1@domain.com/workspace")
				require.NoError(t, err)

				uri, err := workspaceapi.ParseURI("upspin://user1@domain.com/workspace")
				require.NoError(t, err)

				s := new(scheme)
				s.uri = uri
				s.client = newUpspinClient(env.Client)
				s.files = make(map[uintptr]workspaceapi.File)
				// some tests expect /tmp/ to be created and available for write
				_, err = env.Client.MakeDirectory("user1@domain.com/tmp")
				require.NoError(t, err)
				return bindClose{Scheme: s, Env: env}
			})
		})
	})
	/*t.Run("ee packing", func(t *testing.T) {
		test.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
			uri, err := workspaceapi.ParseURI("upspin://ernest@unstable.build/public")
			require.NoError(t, err)
			s, err := newScheme(cfg("ee"), uri)
			require.NoError(t, err)
			return s
		})
	})*/
}
