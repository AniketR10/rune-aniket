// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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

package storagerpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// updateErrService returns a preconfigured (wrapped) error from Update so we
// can verify the server maps wrapped sentinels via errors.Is.
type updateErrService struct {
	storageapi.Service
	err error
}

func (s updateErrService) Update(
	context.Context, string, []storageapi.Update, ...storageapi.Precondition,
) error {
	return s.err
}

func TestServerUpdateMapsWrappedSentinels(t *testing.T) {
	marshaler := docbson.Marshaler()
	updates := []storageapi.Update{{FieldPath: []string{"x"}, Value: "v"}}

	cases := map[string]struct {
		wrapped error
		want    error
	}{
		"wrapped ErrNotFound": {
			wrapped: fmt.Errorf("layer: %w", storageapi.ErrNotFound),
			want:    storageapi.ErrNotFound,
		},
		"wrapped ErrPreconditionFailed": {
			wrapped: fmt.Errorf("layer: %w", storageapi.ErrPreconditionFailed),
			want:    storageapi.ErrPreconditionFailed,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			addr, teardown := runDatastoreServer(t, updateErrService{
				Service: storagestub.NewInMemoryService(),
				err:     tc.wrapped,
			}, marshaler)
			defer teardown()

			client, err := storagerpc.NewClient(addr, marshaler,
				grpc.WithTransportCredentials(insecure.NewCredentials()))
			require.NoError(t, err)
			defer client.Close()

			err = client.Update(context.Background(), "id", updates)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
