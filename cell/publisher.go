package cell

import (
	"unstable.build/go-tui/term"
)

// A cell.Editor that synchronously
// publishes updated to a list of Subscribers
type syncPublisher struct {
	w           Editor
	subscribers []Subscriber
}

func newPublisher(w Editor) *syncPublisher {
	p := new(syncPublisher)
	p.w = w
	return p
}

func (p *syncPublisher) Edit(start, end term.Coordinates, str string) (
	from, to term.Coordinates, old string,
) {
	for _, sub := range p.subscribers {
		sub.OnWillEdit(start, end, str)
	}
	from, to, old = p.w.Edit(start, end, str)
	for _, sub := range p.subscribers {
		sub.OnDidEdit(from, to, old)
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
