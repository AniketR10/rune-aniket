package storage

import "time"

// Document abstracts a document that knows about the ID
// used to index it in the underlying document.Service and
// about the last time that it was updated.
type Document[T any] interface {
	ID() string
	WithID(string) T
	UpdatedTime() time.Time
	WithUpdatedTime(time.Time) T
	WithUpdatedBy(author string) T
}
