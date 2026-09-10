// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
