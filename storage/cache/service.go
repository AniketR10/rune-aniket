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

package cache

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	"unstable.build/go-tui/storage"
)

// Service implements a simple caching document.Service for instances
// that should not perform operations out-of-band. If you know that
// an out-of-band operation has ocurred, you can force refresh the cache
// by calling EvictAll. Callers must employ the List method to internally
// populate the cache otherwise all Get operations will be cache misses.
// Any write evicts all the records in the cache.
type Service[T storage.Document[T]] struct {
	cache  document.Service
	svc    document.Service
	cached atomic.Bool
}

// New allocates storage and initializes a new cache.Service. See Init for more details.
func New[T storage.Document[T]](svc, cache document.Service) *Service[T] {
	ret := new(Service[T])
	ret.Init(svc, cache)
	// perform the ifc satisfaction compile time check here
	// where we have an actual type T that does satisfy Document[T]
	var _ document.Service = ret
	return ret
}

// Init initializes this cache.Service with svc as the underlying service and
// cache as the caching layer.
func (s *Service[T]) Init(svc, cache document.Service) {
	s.svc = svc
	s.cache = cache
}

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
	ctx context.Context, listSvc document.Service, listSvcName string,
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

// Create satisfies document.Service.
func (s *Service[T]) Create(ctx context.Context, ID string, doc interface{}) error {
	err := s.svc.Create(ctx, ID, doc)
	if err == nil && s.cached.Load() {
		if serr := s.cache.Create(ctx, ID, doc); serr != nil {
			log.Errorf("cache set: %v", serr)
		}
	}
	return err
}

// Set satisfies document.Service.
func (s *Service[T]) Set(ctx context.Context, ID string, doc interface{}) error {
	err := s.svc.Set(ctx, ID, doc)
	if err == nil && s.cached.Load() {
		if serr := s.cache.Set(ctx, ID, doc); serr != nil {
			log.Errorf("cache set: %v", serr)
		}
	}
	return err
}

// Update satisfies document.Service.
func (s *Service[T]) Update(ctx context.Context, ID string,
	updates []document.Update, precond ...document.Precondition) error {
	err := s.svc.Update(ctx, ID, updates, precond...)
	if err == nil && s.cached.Load() {
		if uerr := s.cache.Update(ctx, ID, updates, precond...); uerr != nil {
			log.Errorf("cache update: %v", uerr)
		}
	}
	return err
}

// Get satisfies document.Service. Note that doc should be a pointer to T and
// any other type will cause this function to panic.
func (s *Service[T]) Get(ctx context.Context, ID string, doc interface{}) error {
	err := s.cache.Get(ctx, ID, doc)
	if err == nil {
		return nil
	}
	if errors.Is(err, document.ErrNotFound) {
		err = nil
	}
	if err != nil {
		return err
	}
	return s.svc.Get(ctx, ID, doc)
}

// Delete satisfies document.Service.
func (s *Service[T]) Delete(ctx context.Context, ID string) error {
	err := s.svc.Delete(ctx, ID)
	if err == nil && s.cached.Load() {
		if derr := s.cache.Delete(ctx, ID); derr != nil {
			log.Errorf("cache evict: %v", derr)
		}
	}
	return err
}

// List satisfies document.Service.
func (s *Service[T]) List(ctx context.Context, filters []document.Filter) (
	document.Iterator, error,
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

// Close satisfies document.Service.
func (s *Service[T]) Close() (ret error) {
	if err := s.cache.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := s.svc.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}
