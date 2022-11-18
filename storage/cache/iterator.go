package cache

import "unstable.build/go-tui/storage"

type cacheIterator[T storage.Document[T]] struct {
	docs []T
	idx  int
}

func (i *cacheIterator[T]) HasNext() bool {
	return i.idx < len(i.docs)
}

func (i *cacheIterator[T]) NextTo(doc interface{}) error {
	*doc.(*T) = i.docs[i.idx]
	i.idx++
	return nil
}

func (i *cacheIterator[T]) Close() error {
	return nil
}
