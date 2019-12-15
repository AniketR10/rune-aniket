package handler

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/term"
)

type keyMappingHandler struct {
	fractal.Component
	inner    fractal.Handler
	mappings map[term.Event]term.Event
}

// WithMapping takes a handler and a set of event mappings to provide
// key and event mapping to override default handler event handler.
func WithMapping(
	inner fractal.Handler, mappings map[term.Event]term.Event,
) fractal.Handler {
	return keyMappingHandler{inner, inner, mappings}
}

// Handle finds a mapping and overwrites event or delegates the event to
// underlying handler.
func (k keyMappingHandler) Handle(ev term.Event) bool {
	mapped, ok := k.mappings[ev]
	if ok {
		ev = mapped
	}
	return k.inner.Handle(ev)
}

// Cursor delegates call to underlying handler.
func (k keyMappingHandler) Cursor() (term.Coordinates, bool) {
	return k.inner.Cursor()
}

// Man returns remapped Manual from underlying handler.
func (k keyMappingHandler) Man() fractal.Manual {
	m := k.inner.Man()
	for from, to := range k.mappings {
		m.Keys[to] = m.Keys[from]
	}
	return m
}
