package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/ernestrc/blue/document"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/storage"
)

// Service implements a simple caching document.Service for instances
// that should not perform operations out-of-band. If you know that
// an out-of-band operation has ocurred, you can force refresh the cache
// by calling EvictAll. Callers must employ the List method to internally
// populate the cache otherwise all Get operations will be cache misses.
// Any write evicts all the records in the cache.
type Service[T storage.Document[T]] struct {
	cache document.Service
	svc   document.Service
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
	if ret != nil {
		return ret
	}
	return nil
}

// Create satisfies document.Service.
func (s *Service[T]) Create(ctx context.Context, ID string, doc interface{}) error {
	err := s.svc.Create(ctx, ID, doc)
	if err == nil {
		err = s.EvictAll(ctx)
		if err != nil {
			log.Errorf("EvictAll: %v", err)
		}
	}
	return err
}

// Set satisfies document.Service.
func (s *Service[T]) Set(ctx context.Context, ID string, doc interface{}) error {
	err := s.svc.Set(ctx, ID, doc)
	if err == nil {
		err = s.EvictAll(ctx)
		if err != nil {
			log.Errorf("EvictAll: %v", err)
		}
	}
	return err
}

// Update satisfies document.Service.
func (s *Service[T]) Update(ctx context.Context, ID string,
	updates []document.Update, precond ...document.Precondition) error {
	err := s.svc.Update(ctx, ID, updates, precond...)
	if err == nil {
		err = s.EvictAll(ctx)
		if err != nil {
			log.Errorf("EvictAll: %v", err)
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
	if err == nil {
		err = s.EvictAll(ctx)
		if err != nil {
			log.Errorf("EvictAll: %v", err)
		}
	}
	return err
}

// List satisfies document.Service.
func (s *Service[T]) List(ctx context.Context, filters []document.Filter) (
	document.Iterator, error,
) {
	it, err := s.cache.List(ctx, filters)
	if err == nil && it.HasNext() {
		return it, err
	}

	it, err = s.svc.List(ctx, filters)
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
		err = fmt.Errorf("EvictAll: %w", err)
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
