package cache

import (
	"context"
	"fmt"

	"github.com/ernestrc/blue/document"
	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/storage"
)

// Service implements a simple caching document.Service for instances
// that should not perform operations out-of-band. If you know that
// an out-of-band operation has ocurred, you can force refresh the cache
// by calling EvictAll.
type Service[T storage.Document[T]] struct {
	cache      document.Service
	svc        document.Service
	listCached bool
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

func (s *Service[T]) EvictAll() error {
	ctx := context.Background()
	it, err := s.cache.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("List: %v", err)
	}
	var ret error
	for it.HasNext() {
		var temp T
		err := it.NextTo(&temp)
		if err != nil {
			ret = multierr.Append(ret, fmt.Errorf("NextTo: %v", err))
			continue
		}
		err = s.cache.Delete(ctx, temp.ID())
		if err != nil && err != document.ErrNotFound {
			ret = multierr.Append(ret, fmt.Errorf("Delete: %v", err))
		}
	}
	if ret != nil {
		return ret
	}
	s.listCached = false
	return nil
}

func (s *Service[T]) evict(ctx context.Context, ID string) error {
	s.listCached = false
	err := s.cache.Delete(ctx, ID)
	if err == document.ErrNotFound {
		err = nil
	}
	return err
}

// Create satisfies document.Service.
func (s *Service[T]) Create(ctx context.Context, ID string, doc interface{}) error {
	err := s.svc.Create(ctx, ID, doc)
	if err == nil {
		err = s.evict(ctx, ID)
	}
	return err
}

// Set satisfies document.Service.
func (s *Service[T]) Set(ctx context.Context, ID string, doc interface{}) error {
	err := s.svc.Set(ctx, ID, doc)
	if err == nil {
		err = s.evict(ctx, ID)
	}
	return err
}

// Update satisfies document.Service.
func (s *Service[T]) Update(ctx context.Context, ID string,
	updates []document.Update, precond ...document.Precondition) error {
	err := s.svc.Update(ctx, ID, updates, precond...)
	if err == nil {
		err = s.evict(ctx, ID)
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
	if err == document.ErrNotFound {
		err = nil
	}
	if err != nil {
		return err
	}
	err = s.svc.Get(ctx, ID, doc)
	if err != nil {
		return err
	}
	return s.cache.Set(ctx, ID, doc)
}

// Delete satisfies document.Service.
func (s *Service[T]) Delete(ctx context.Context, ID string) error {
	err := s.svc.Delete(ctx, ID)
	if err == nil {
		err = s.evict(ctx, ID)
	}
	return err
}

// List satisfies document.Service.
func (s *Service[T]) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	if s.listCached {
		return s.cache.List(ctx, filters)
	}

	it, err := s.svc.List(ctx, filters)
	if err != nil {
		return nil, err
	}

	// make it simple
	if len(filters) != 0 {
		return it, nil
	}

	defer it.Close()

	err = s.EvictAll()
	if err != nil {
		return nil, fmt.Errorf("EvictAll: %v", err)
	}

	docs := make([]T, 0)
	for it.HasNext() {
		var temp T
		err = it.NextTo(&temp)
		if err != nil {
			break
		}
		docs = append(docs, temp)

		err = s.cache.Create(ctx, temp.ID(), temp)
		if err != nil {
			break
		}
	}
	if err != nil {
		// evict all if we fail to populate the cache
		everr := s.EvictAll()
		if everr != nil {
			err = multierr.Append(err, everr)
		}
		return nil, err
	}
	s.listCached = true
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
