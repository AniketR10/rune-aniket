package cell

import (
	"github.com/ernestrc/fractal/term"
)

// A cell.Writer that synchronously
// publishes updated to a list of Subscribers
type syncPublisher struct {
	w           Writer
	subscribers []Subscriber
}

func newPublisher(w Writer) *syncPublisher {
	p := new(syncPublisher)
	p.w = w
	return p
}

func (p *syncPublisher) Delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	start, end, str = p.w.Delete(from, to)
	for _, sub := range p.subscribers {
		sub.OnDelete(start, end, str)
	}
	return
}

func (p *syncPublisher) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	from, to = p.w.Insert(at, str)
	for _, sub := range p.subscribers {
		sub.OnInsert(from, to, str)
	}
	return
}

func (p *syncPublisher) subscribe(s Subscriber) {
	p.subscribers = append(p.subscribers, s)
}
