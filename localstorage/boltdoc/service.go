// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

// Package boltdoc implements a storageapi.Service backed by bbolt.
//
// It wraps a blue/document/bolt.Store via the ox-api bluestore.AdaptTo
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

	bluebolt "github.com/unstablebuild/blue/document/bolt"
	"github.com/unstablebuild/ox-api/bluestore"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"

	bluedoc "github.com/unstablebuild/blue/document"
	bluemarshal "github.com/unstablebuild/blue/document/docmarshal"
)

// ErrClosing is returned to inflight write requests
// before committing changes to disk if the service is currently closing.
// firstmover compares this sentinel against returned errors to decide
// whether to fall back to a remote leader.
var ErrClosing = errors.New("service is closing")

// defaultCollectionID is the bucket name used for the root, unpartitioned
// collection of the bolt-backed storage. Partitions create sibling buckets.
const defaultCollectionID = "rune"

// New opens (or creates) the bolt database at dbPath and returns a
// storageapi.Service backed by it. The marshaler is used to encode every
// stored document and to drive UpdateProto field-path lowering; it must
// be the same marshaler the wire RPC layer uses to encode precondition
// values, otherwise CAS preconditions break due to Go's type-strict ==.
func New(dbPath string, marshaler docmarshal.Marshaler) (
	storageapi.Service, *Service, error,
) {
	store, err := bluebolt.NewWithMarshaler(dbPath, defaultCollectionID, marshaler)
	if err != nil {
		return nil, nil, fmt.Errorf("open bolt db at %q: %w", dbPath, err)
	}
	svc := &Service{
		dbPath:    dbPath,
		marshaler: marshaler,
		store:     store,
	}
	root := rootStore{Store: store, dbPath: dbPath, marshaler: marshaler}
	return bluestore.AdaptTo(root), svc, nil
}

// Service is the concrete handle to a bolt-backed storageapi.Service.
// Callers should use the storageapi.Service returned by New for normal
// operations; this handle exposes Close so resource ownership is explicit.
type Service struct {
	dbPath    string
	marshaler bluemarshal.Marshaler
	store     *bluebolt.Store
}

// Close releases the underlying bolt database.
func (s *Service) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

type rootStore struct {
	*bluebolt.Store
	dbPath    string
	marshaler bluemarshal.Marshaler
}

func (p rootStore) Partition(name string) (bluedoc.Service, error) {
	if name == "" {
		return nil, errors.New("invalid partition: empty")
	}
	store, err := bluebolt.NewWithMarshaler(p.dbPath, name, p.marshaler)
	if err != nil {
		return nil, fmt.Errorf("open partition %q: %w", name, err)
	}
	return partitionStore{Store: store, dbPath: p.dbPath, marshaler: p.marshaler}, nil
}

type partitionStore struct {
	*bluebolt.Store
	dbPath    string
	marshaler bluemarshal.Marshaler
}

func (p partitionStore) Close() error { return nil }

func (p partitionStore) Partition(name string) (bluedoc.Service, error) {
	if name == "" {
		return nil, errors.New("invalid partition: empty")
	}
	store, err := bluebolt.NewWithMarshaler(p.dbPath, name, p.marshaler)
	if err != nil {
		return nil, fmt.Errorf("open partition %q: %w", name, err)
	}
	return partitionStore{Store: store, dbPath: p.dbPath, marshaler: p.marshaler}, nil
}
