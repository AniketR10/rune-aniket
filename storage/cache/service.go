package cache

import (
	"context"

	"github.com/ernestrc/blue/document"
	"unstable.build/go-tui/storage"
)

// Service implements a simple caching document.Service for instances
// that should not perform operations out-of-band. If you know that
// an out-of-band operation has ocurred, you can force refresh the cache
// by calling EvictAll.
type Service[T storage.Document[T]] struct {
	svc    document.Service
	cache  map[string]T
	listed bool
}

// New allocates storage and initializes a new cache.Service.
func New[T storage.Document[T]](svc document.Service) *Service[T] {
	ret := new(Service[T])
	ret.Init(svc)
	// perform the ifc satisfaction compile time check here
	// where we have an actual type T that does satisfy Document[T]
	var _ document.Service = ret
	return ret
}

// Init initializes this cache.Service with svc.
func (s *Service[T]) Init(svc document.Service) {
	s.svc = svc
	s.cache = make(map[string]T)
}

func (s *Service[T]) EvictAll() error {
	// go's compiler optimizes this, so we don't need
	// to allocate a new map.
	for k := range s.cache {
		delete(s.cache, k)
	}
	s.listed = false
	return nil
}

func (s *Service[T]) evict(ID string) {
	s.listed = false
	delete(s.cache, ID)
}

// Create satisfies document.Service.
func (s *Service[T]) Create(ctx context.Context, ID string, doc interface{}) error {
	err := s.svc.Create(ctx, ID, doc)
	if err == nil {
		s.evict(ID)
	}
	return err
}

// Set satisfies document.Service.
func (s *Service[T]) Set(ctx context.Context, ID string, doc interface{}) error {
	err := s.svc.Set(ctx, ID, doc)
	if err == nil {
		s.evict(ID)
	}
	return err
}

// Update satisfies document.Service.
func (s *Service[T]) Update(ctx context.Context, ID string,
	updates []document.Update, precond ...document.Precondition) error {
	err := s.svc.Update(ctx, ID, updates, precond...)
	if err == nil {
		s.evict(ID)
	}
	return err
}

// Get satisfies document.Service. Note that doc should be a pointer to T and
// any other type will cause this function to panic.
func (s *Service[T]) Get(ctx context.Context, ID string, doc interface{}) error {
	if t, ok := s.cache[ID]; ok {
		*doc.(*T) = t
		return nil
	}
	err := s.svc.Get(ctx, ID, doc)
	if err != nil {
		return err
	}
	s.cache[ID] = *doc.(*T)
	return nil
}

// Delete satisfies document.Service.
func (s *Service[T]) Delete(ctx context.Context, ID string) error {
	err := s.svc.Delete(ctx, ID)
	if err == nil {
		s.evict(ID)
	}
	return nil
}

// List satisfies document.Service. It doesn't return cached results if filters are passed.
func (s *Service[T]) List(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
	if s.listed && len(filters) == 0 {
		docs := make([]T, 0, len(s.cache))
		for _, v := range s.cache {
			docs = append(docs, v)
		}
		return &cacheIterator[T]{docs: docs}, nil
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

	s.EvictAll()
	docs := make([]T, 0, len(s.cache)) // still a good approximation
	for it.HasNext() {
		var temp T
		err := it.NextTo(&temp)
		if err != nil {
			return nil, err
		}
		s.cache[temp.ID()] = temp
		docs = append(docs, temp)
	}
	s.listed = true
	return &cacheIterator[T]{docs: docs}, nil
}

// Close satisfies document.Service.
func (s *Service[T]) Close() error {
	s.cache = nil
	return s.svc.Close()
}
