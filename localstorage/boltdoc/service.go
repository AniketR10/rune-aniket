// Copyright (C) 2017-2026 Unstable Build, LLC
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

// Package boltdoc implements a storageapi.Service backed by bbolt.
//
// It wraps a blue/document/bolt.Store via the bluestore.AdaptTo
// adapter so that all CRUD, List, and CAS Update operations (including
// the Version-equality preconditions used by storageapi.ConsistentUpdate)
// run inside a single bbolt RW transaction, providing serializable
// read-modify-write semantics that the prior schemedoc backend lacked.
//
// The injected marshaler must match the wire marshaler used elsewhere
// (rune-agent extension, IDE host) so precondition Value types decoded
// from the wire round-trip identically against fields decoded from
// storage; Go's == is type-strict.
package boltdoc

import (
	"errors"
	"fmt"
	"path/filepath"

	bluebolt "github.com/unstablebuild/blue/document/bolt"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"unstable.build/rune/localstorage/bluestore"

	bluedoc "github.com/unstablebuild/blue/document"
	bluemarshal "github.com/unstablebuild/blue/document/docmarshal"
)

// ErrClosing is returned to inflight write requests
// before committing changes to disk if the service is currently closing.
// firstmover compares this sentinel against returned errors to decide
// whether to fall back to a remote leader.
var ErrClosing = errors.New("service is closing")

const defaultCollectionID = "default"

// New opens (or creates) the bolt database at dbPath and returns a
// storageapi.Service backed by it. The marshaler is used to encode every
// stored document and to drive UpdateProto field-path lowering; it must
// be the same marshaler the wire RPC layer uses to encode precondition
// values, otherwise CAS preconditions break due to Go's type-strict ==.
func New(dbPath string, marshaler docmarshal.Marshaler) (
	storageapi.Service, error,
) {
	store, err := bluebolt.NewWithMarshaler(dbPath, defaultCollectionID, marshaler)
	if err != nil {
		return nil, fmt.Errorf("open bolt db at %q: %w", dbPath, err)
	}
	root := rootStore{Store: store, dbPath: dbPath, marshaler: marshaler}
	return bluestore.AdaptTo(root), nil
}

type rootStore struct {
	*bluebolt.Store
	partition string
	dbPath    string
	marshaler bluemarshal.Marshaler
}

func (p rootStore) Partition(name string) (bluedoc.Service, error) {
	if name == "" {
		return nil, errors.New("invalid partition: empty")
	}
	partitionName := filepath.Join(p.partition, name)
	store, err := bluebolt.NewWithMarshaler(p.dbPath, partitionName, p.marshaler)
	if err != nil {
		return nil, fmt.Errorf("open partition %q: %w", name, err)
	}
	return rootStore{
		Store:     store,
		partition: partitionName,
		dbPath:    p.dbPath,
		marshaler: p.marshaler,
	}, nil
}
