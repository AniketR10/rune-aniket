package util

import (
	"context"

	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
)

// ResourceTracker implements a remote text editor tracker,
// which handles events emitted by a textapi.Editor
// to replicate the exact state of all the open resources.
//
// It satisfies textapi.EventHandler so clients are
// responsible for subscribing it to a textapi.Editor.
//
// Clients must subscribe to EventTypeFocus if
// wrap mode is set, and cursor position needs
// to be tracked.
type ResourceTracker struct {
	resources map[string]*TrackedResource
	wrap      bool
	tabspaces int
	focus     *TrackedResource
}

var _ textapi.EventHandler = (*ResourceTracker)(nil)

// ResourceTrackerEventsComplete returns a slice of textapi.EventType
// required in calls to textapi.Editor.SubscribeEvents for a ResourceTracker
// to replicate all state, which includes, cursor and scroll offsets.
//
// Subscribing to cursor and scroll events might impose a performance
// penalty, so use only when indeed cursor and scroll coordinates
// are strictly necessary.
func ResourceTrackerEventsComplete() []textapi.EventType {
	return []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeFocus,
		textapi.EventTypeClose,
		textapi.EventTypeFlush,
		textapi.EventTypeEdit,
		textapi.EventTypeScroll,
		textapi.EventTypeCursor,
	}
}

// ResourceTrackerEventsContent returns a slice of textapi.EventType
// required in calls to textapi.Editor.SubscribeEvents for a ResourceTracker
// to replicate only the resource content, edit by edit. Cursor and
// scroll offsets will not be tracked.
func ResourceTrackerEventsContent() []textapi.EventType {
	return []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeFocus,
		textapi.EventTypeClose,
		textapi.EventTypeFlush,
		textapi.EventTypeEdit,
	}
}

// ResourceTrackerEventsContentFlush returns a slice of textapi.EventType
// required in calls to textapi.Editor.SubscribeEvents for a ResourceTracker
// to replicate only the resource content, upon flushing. Edits before
// flushing to disk are ignored and also scroll and cursor offsets
// will not be tracked.
func ResourceTrackerEventsFlushOnly() []textapi.EventType {
	return []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeFocus,
		textapi.EventTypeClose,
		textapi.EventTypeFlush,
	}
}

// NewResourceTracker allocates storage for a new tracker and initializes it.
func NewResourceTracker(tabspaces int, wrap bool) *ResourceTracker {
	ret := new(ResourceTracker)
	ret.Init(tabspaces, wrap)
	return ret
}

// Init initializes this ResourceTracker.
func (t *ResourceTracker) Init(tabspaces int, wrap bool) {
	t.resources = make(map[string]*TrackedResource)
	t.tabspaces = tabspaces
	t.wrap = wrap
}

// Resource returns the tracked resource with the given URI, or false
// if a resource with the given uri is not currently open.
func (t *ResourceTracker) Resource(uri workspaceapi.URI) (*TrackedResource, bool) {
	ret, ok := t.resources[uri.String()]
	return ret, ok
}

// Focus returns the tracked resource currently in focus, or false
// if there is no tracked resource currently in focus. Clients
// must subscribe this ResourceTracker to EventTypeFocus events.
func (t *ResourceTracker) Focus() (*TrackedResource, bool) {
	ok := t.focus != nil
	return t.focus, ok
}

// Handle satisfies textapi.EventHandler.
func (h *ResourceTracker) Handle(_ context.Context, ev textapi.Event) (exit bool) {
	if ev.URI == (workspaceapi.URI{}) {
		h.log(log.TraceLevel, "ignoring event with resource with empty uri: %d", ev.Type)
		return
	}
	var ok bool
	switch ev.Type {
	case textapi.EventTypeOpen:
		h.handleResourceOpen(ev)
		ok = true
	case textapi.EventTypeFocus:
		ok = h.handleResourceFocus(ev)
	case textapi.EventTypeClose:
		h.handleResourceClose(ev)
		ok = true
	case textapi.EventTypeEdit:
		ok = h.handleResourceEdit(ev)
	case textapi.EventTypeFlush:
		ok = h.handleResourceFlush(ev)
	case textapi.EventTypeScroll:
		ok = h.handleResourceScroll(ev)
	case textapi.EventTypeCursor:
		ok = h.handleResourceCursor(ev)
	default:
		ok = true // ignore the rest of event types
	}

	if !ok {
		h.log(log.WarnLevel, "resource with uri '%s' not found "+
			"or could not process event type: %d", ev.URI.String(), ev.Type)
	}
	return
}

func (h *ResourceTracker) handleResourceOpen(ev textapi.Event) {
	buf := new(cell.Buffer)
	buf.InitWithTabspaces(h.tabspaces)
	buf.WriteString(ev.Content)
	trackedResource := new(TrackedResource)
	trackedResource.uri = ev.URI
	trackedResource.Scroll.Init(buf)
	trackedResource.Scroll.Wrap = h.wrap
	trackedResource.cursor.Init(&trackedResource.Scroll)

	h.resources[ev.URI.String()] = trackedResource
}

func (h *ResourceTracker) handleResourceFocus(ev textapi.Event) bool {
	res, ok := h.resources[ev.URI.String()]
	if !ok {
		return false
	}

	res.Scroll.Resize(ev.Start.X, ev.Start.Y)
	if res.Scroll.Wrap {
		res.Scroll.RecalculateWraps()
	}
	h.focus = res
	return true
}

func (h *ResourceTracker) handleResourceClose(ev textapi.Event) {
	delete(h.resources, ev.URI.String())
}

func (h *ResourceTracker) handleResourceEdit(ev textapi.Event) bool {
	res, ok := h.resources[ev.URI.String()]
	if !ok {
		return false
	}
	res.Scroll.Buffer().Edit(ev.Start, ev.End, ev.Content)
	if res.Scroll.Wrap {
		res.Scroll.RecalculateWraps()
	}
	return true
}

func (h *ResourceTracker) handleResourceFlush(ev textapi.Event) bool {
	res, ok := h.resources[ev.URI.String()]
	if !ok {
		return false
	}
	res.Scroll.Buffer().Reset()
	res.Scroll.Buffer().WriteString(ev.Content)
	if res.Scroll.Wrap {
		res.Scroll.RecalculateWraps()
	}
	return true
}

func (h *ResourceTracker) handleResourceScroll(ev textapi.Event) bool {
	res, ok := h.resources[ev.URI.String()]
	if !ok {
		return false
	}
	if res.Scroll.Offset() == ev.Start {
		return true
	}
	return res.Scroll.SetOffset(ev.Start)
}

func (h *ResourceTracker) handleResourceCursor(ev textapi.Event) bool {
	res, ok := h.resources[ev.URI.String()]
	if !ok {
		return false
	}
	_, ok = res.cursor.MoveToScroll(ev.From)
	return ok
}

func (h *ResourceTracker) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		logging.KeyClass: "extutil.ResourceTracker",
	}).Logf(level, msg, args...)
}
