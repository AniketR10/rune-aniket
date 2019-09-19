package fractal

import "termbox"

type keyMappingHandler struct {
	Component
	inner    Handler
	mappings map[termbox.Event]termbox.Event
}

// WithMapping takes a handler and a set of event mappings to provide
// key and event mapping to override default handler event handler.
func WithMapping(inner Handler, mappings map[termbox.Event]termbox.Event) Handler {
	return keyMappingHandler{inner, inner, mappings}
}

// Handle finds a mapping and overwrites event or delegates the event to
// underlying handler.
func (k keyMappingHandler) Handle(ev termbox.Event) bool {
	mapped, ok := k.mappings[ev]
	if ok {
		ev = mapped
	}
	return k.inner.Handle(ev)
}

// GetCursor delegates call to underlying handler.
func (k keyMappingHandler) GetCursor() Coordinates {
	return k.inner.GetCursor()
}

// Man returns remapped Manual from underlying handler.
func (k keyMappingHandler) Man() Manual {
	m := k.inner.Man()
	for from, to := range k.mappings {
		m.Keys[to] = m.Keys[from]
	}
	return m
}
