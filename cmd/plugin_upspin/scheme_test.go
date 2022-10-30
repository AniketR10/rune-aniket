package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/test"
	"upspin.io/test/testenv"
	"upspin.io/upspin"
)

func TestScheme(t *testing.T) {
	t.Run("plain packing", func(t *testing.T) {
		t.Run("root of path", func(t *testing.T) {
			test.TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
				user := "user1@domain.com"
				uri, err := workspace.ParseURI("upspin://" + user)
				require.NoError(t, err)
				setup := &testenv.Setup{
					OwnerName: upspin.UserName(user),
					Kind:      "inprocess",
					Packing:   upspin.PlainPack,
				}
				env, err := testenv.New(setup)
				require.NoError(t, err)
				s := new(scheme)
				s.uri = uri
				s.client = newUpspinClient(env.Client)
				// some tests expect /tmp/ to be created and available for write
				_, err = env.Client.MakeDirectory("user1@domain.com/tmp")
				require.NoError(t, err)
				return s
			})
		})
	})
	/*t.Run("ee packing", func(t *testing.T) {
		test.TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
			uri, err := workspace.ParseURI("upspin://ernest@unstable.build/public")
			require.NoError(t, err)
			s, err := newScheme(cfg("ee"), uri)
			require.NoError(t, err)
			return s
		})
	})*/
}
