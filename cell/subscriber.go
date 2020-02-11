package cell

import (
	"github.com/ernestrc/fractal/term"
)

// Subscriber is the interface that wraps methods to receive to updates to
// an underlying cell.Writer.
//
// Subscribers MUST NOT have mutable access to the underlying cell.Writer
// they're subscribing to, as updates are published synchronously so
// program could enter in an infinite loop.
//
// Subscribers constructors SHOULD subscribe to a Publisher upon initialization.
type Subscriber interface {
	OnWillInsert(at term.Coordinates, str string)
	OnDidInsert(from, to term.Coordinates)
	OnWillDelete(from, to term.Coordinates)
	OnDidDelete(start, end term.Coordinates, str string)
	Unsubscribe()
}

// Publisher is the interface that wraps the Subscribe method.
//
// Subscribe enables subscription of insert/delete events. See Subscriber.
type Publisher interface {
	Subscribe(Subscriber)
	Unsubscribe(Subscriber)
}

// PublisherReader is the interface that groups Publisher and Reader.
//
// This interface should be used within Subscribers which need to read from
// a Reader when handling updates. See Subscriber.
type PublisherReader interface {
	Publisher
	Reader
}
