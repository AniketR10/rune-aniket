package cell

import (
	"github.com/ernestrc/go-tui/term"
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
	for _, sub := range p.subscribers {
		sub.OnWillDelete(from, to)
	}
	start, end, str = p.w.Delete(from, to)
	for _, sub := range p.subscribers {
		sub.OnDidDelete(start, end, str)
	}
	return
}

func (p *syncPublisher) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	for _, sub := range p.subscribers {
		sub.OnWillInsert(at, str)
	}
	from, to = p.w.Insert(at, str)
	for _, sub := range p.subscribers {
		sub.OnDidInsert(from, to)
	}
	return
}

func (p *syncPublisher) Subscribe(s Subscriber) {
	p.subscribers = append(p.subscribers, s)
}

func (p *syncPublisher) Unsubscribe(s Subscriber) {
	unsubs := -1
	for i, sub := range p.subscribers {
		if sub == s {
			unsubs = i
			break
		}
	}
	if unsubs < 0 {
		panic("Subscriber is not subscribed")
	}
	p.subscribers = append(p.subscribers[:unsubs], p.subscribers[unsubs+1:]...)
}
