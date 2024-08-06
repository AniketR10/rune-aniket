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

package test_all

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document/firestore"
	"github.com/unstablebuild/blue/encoding/json"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace/docscheme"
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
		scheme, err := docscheme.Scheme[testStruct](workspaceURI, svc,
			json.Marshaler(), errMissingID, "author")(ctx, config.NopConfig(), workspaceURI)
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
		scheme, err := docscheme.Scheme[testStruct](workspaceURI, svc,
			json.Marshaler(), errMissingID, "author")(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		f, werr := scheme.Open("/.something.swp", os.O_CREATE, 0)
		require.Nil(t, werr)
		require.NoError(t, f.Close())
	})
}
