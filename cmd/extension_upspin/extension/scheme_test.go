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
	"testing"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/stretchr/testify/require"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace/workspacetest"
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
			workspacetest.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
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
		workspacetest.TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
			uri, err := workspaceapi.ParseURI("upspin://ernest@unstable.build/public")
			require.NoError(t, err)
			s, err := newScheme(cfg("ee"), uri)
			require.NoError(t, err)
			return s
		})
	})*/
}
