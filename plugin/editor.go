package plugin

/* TODO

// LocationLister is the interface that wraps methods to set an
// editor's LocationList.
type LocationLister interface {
	SetLocationList(editor.LocationList) error
}

// Reader groups methods to read the content of an editor's buffer.
type Reader interface {
	Rows() (int, error)
	Columns(row int) (int, error)
	Cell(term.Coordinates) (term.Cell, bool, error)
	RawCells() ([][]term.Cell, error)
	String() (string, error)
}

// Writer groups methods to write to an editor's buffer.
type Writer interface {
	Insert(at term.Coordinates, str string) (from, to term.Coordinates, err error)
	Delete(from, to term.Coordinates) (start, end term.Coordinates, str string, err error)
}

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

// Publisher is the interface that wraps methods to subscribe, unsubscribe
// from content updates.
type Publisher interface {
	Subscribe(Subscriber) error
	Unsubscribe(Subscriber) error
}

// Editor is an interface that groups methods to manipulate a text editor.
type Editor interface {
	LocationLister
	KeyMapper
	Reader
	Writer
}
*/
