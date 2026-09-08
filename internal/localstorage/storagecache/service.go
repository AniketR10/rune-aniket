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

package storagecache

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/rune/internal/localstorage"
)

// Service implements a simple caching storageapi.Service for instances
// that should not perform operations out-of-band. If you know that
// an out-of-band operation has ocurred, you can force refresh the cache
// by calling EvictAll. Callers must employ the List method to internally
// populate the cache otherwise all Get operations will be cache misses.
// Any write evicts all the records in the cache.
type Service[T localstorage.Document[T]] struct {
	cache  storageapi.Service
	svc    storageapi.Service
	cached atomic.Bool
}

// New allocates storage and initializes a new cache.Service. See Init for more details.
func New[T localstorage.Document[T]](svc, cache storageapi.Service) *Service[T] {
	ret := new(Service[T])
	ret.Init(svc, cache)
	// perform the ifc satisfaction compile time check here
	// where we have an actual type T that does satisfy Document[T]
	var _ storageapi.Service = ret
	return ret
}

// Init initializes this cache.Service with svc as the underlying service and
// cache as the caching layer.
func (s *Service[T]) Init(svc, cache storageapi.Service) {
	s.svc = svc
	s.cache = cache
}

// EvictAll evicts all documents from this cache.
func (s *Service[T]) EvictAll(ctx context.Context) error {
	err := s.evictAll(ctx, s.cache, "cache")
	if err != nil {
		// if we fail to remove evict records from cache
		// try to list using the underlying service
		if serr := s.evictAll(ctx, s.svc, "service"); serr != nil {
			err = multierr.Append(err, serr)
		}
		return err
	}
	return nil
}

func (s *Service[T]) evictAll(
	ctx context.Context, listSvc storageapi.Service, listSvcName string,
) error {
	it, err := listSvc.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("List: %w", err)
	}
	var ret error
	for it.HasNext() {
		var temp T
		err := it.NextTo(&temp)
		if err != nil {
			ret = multierr.Append(ret, fmt.Errorf("NextTo: %w", err))
			continue
		}
		if temp.ID() == "" {
			ret = multierr.Append(ret, fmt.Errorf("empty document ID: evict using '%s'", listSvcName))
			continue
		}
		err = s.cache.Delete(ctx, temp.ID())
		if err != nil {
			ret = multierr.Append(ret, fmt.Errorf("Delete: %w", err))
			continue
		}
	}
	s.cached.Store(false)
	return ret
}

// Create satisfies storageapi.Service.
func (s *Service[T]) Create(ctx context.Context, ID string, doc any) error {
	err := s.svc.Create(ctx, ID, doc)
	if err == nil && s.cached.Load() {
		if serr := s.cache.Create(ctx, ID, doc); serr != nil {
			log.Errorf("cache set: %v", serr)
		}
	}
	return err
}

// Set satisfies storageapi.Service.
func (s *Service[T]) Set(ctx context.Context, ID string, doc any) error {
	err := s.svc.Set(ctx, ID, doc)
	if err == nil && s.cached.Load() {
		if serr := s.cache.Set(ctx, ID, doc); serr != nil {
			log.Errorf("cache set: %v", serr)
		}
	}
	return err
}

// Update satisfies storageapi.Service.
func (s *Service[T]) Update(ctx context.Context, ID string,
	updates []storageapi.Update, precond ...storageapi.Precondition) error {
	err := s.svc.Update(ctx, ID, updates, precond...)
	if err == nil && s.cached.Load() {
		if uerr := s.cache.Update(ctx, ID, updates, precond...); uerr != nil {
			log.Errorf("cache update: %v", uerr)
		}
	}
	return err
}

// Get satisfies storageapi.Service. Note that doc should be a pointer to T and
// any other type will cause this function to panic.
func (s *Service[T]) Get(ctx context.Context, ID string, doc any) error {
	err := s.cache.Get(ctx, ID, doc)
	if err == nil {
		return nil
	}
	if errors.Is(err, storageapi.ErrNotFound) {
		err = nil
	}
	if err != nil {
		return err
	}
	return s.svc.Get(ctx, ID, doc)
}

// Delete satisfies storageapi.Service.
func (s *Service[T]) Delete(ctx context.Context, ID string) error {
	err := s.svc.Delete(ctx, ID)
	if err == nil && s.cached.Load() {
		if derr := s.cache.Delete(ctx, ID); derr != nil {
			log.Errorf("cache evict: %v", derr)
		}
	}
	return err
}

// List satisfies storageapi.Service.
func (s *Service[T]) List(ctx context.Context, filters []storageapi.Filter) (
	storageapi.Iterator, error,
) {
	if s.cached.Load() {
		it, err := s.cache.List(ctx, filters)
		if err == nil && it.HasNext() {
			return it, err
		} else if err != nil {
			log.Errorf("list from cache: %v", err)
		}
	}

	s.cached.Store(false)

	it, err := s.svc.List(ctx, filters)
	if err != nil {
		return nil, err
	}

	// make it simple
	if len(filters) != 0 {
		return it, nil
	}

	defer it.Close()

	err = s.EvictAll(ctx)
	if err != nil {
		err = fmt.Errorf("cache evict all: %w", err)
		log.Error(err)
		return nil, err
	}

	docs := make([]T, 0)
	for it.HasNext() {
		var temp T
		err = it.NextTo(&temp)
		if err != nil {
			break
		}
		docs = append(docs, temp)

		// in theory Create should never fail since we have
		// just evicted all records in practice if a record
		// is malformed and ID returns a non-unique string
		// then we are never able to populate the cache
		err = s.cache.Set(ctx, temp.ID(), temp)
		if err != nil {
			break
		}
	}
	if err != nil {
		log.Warnf("could not populate cache after list: %v", err)
		// evict all if we fail to populate the cache
		everr := s.EvictAll(ctx)
		if everr != nil {
			log.Errorf("EvictAll: %v", err)
			err = multierr.Append(err, everr)
		}
		return nil, err
	}
	s.cached.Store(true)
	return &cacheIterator[T]{docs: docs}, nil
}

// Partition returns a partitioned cache service over matching underlying partitions.
func (s *Service[T]) Partition(name string) (storageapi.Service, error) {
	svc, err := s.svc.Partition(name)
	if err != nil {
		return nil, err
	}
	cache, err := s.cache.Partition(name)
	if err != nil {
		return nil, err
	}
	return New[T](svc, cache), nil
}

// Close satisfies storageapi.Service.
func (s *Service[T]) Close() (ret error) {
	if err := s.cache.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := s.svc.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}
