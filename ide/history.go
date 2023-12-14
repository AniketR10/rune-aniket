package ide

import (
	"context"
	"sort"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/text"
)

var (
	historyEventInterests = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeFlush,
		textapi.EventTypeEdit,
	}
)

type file struct {
	Dirty     bool
	UpdatedAt time.Time
	URIString string
}

type cache struct {
	Files map[string]file
}

type history struct {
	svc   document.Service
	cache map[string]cache
}

func newHistory(storage document.Service) *history {
	ret := &history{
		svc:   storage,
		cache: make(map[string]cache),
	}
	return ret
}

func (h *history) log(level log.Level, msg string, args ...interface{}) {
	log.WithFields(log.Fields{logging.KeyClass: "ide.history"}).
		Logf(level, msg, args...)
}

func (h *history) loadWorkspaceData(uri workspaceapi.URI, restore bool) {
	const cacheSetTimeout = 1 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), cacheSetTimeout)
	defer cancel()

	uriStr := uri.String()

	var workspace cache
	err := h.svc.Get(ctx, uriStr, &workspace)
	if err != nil && err != document.ErrNotFound {
		log.WithFields(log.Fields{logging.KeyClass: "ide.history"}).
			Warnf("could not persist updated cache to durable storage: %v", err)
	}

	if !restore || err != nil {
		h.log(log.TraceLevel, "reseting cache for workspace: %s", uriStr)
		h.resetWorkspaceCache(uri)
	} else {
		h.cache[uriStr] = workspace
	}
	h.log(log.TraceLevel, "loaded workspace %s cache from storage: %v", uriStr, h.cache)
}

func (h *history) recordAddWorkspace(
	uri workspaceapi.URI, ed text.Editor, restore bool,
) (ret []file) {
	h.loadWorkspaceData(uri, restore)

	uriStr := uri.String()
	err := ed.SubscribeEvents(historyEventInterests,
		&workspaceHistory{svc: h.svc, uri: uriStr, cache: h.cache})
	if err != nil {
		h.log(log.ErrorLevel, "could not subscribe to file events: %v", err)
	}
	mapFiles := h.cache[uriStr].Files
	ret = make([]file, 0, len(h.cache))
	for _, f := range mapFiles {
		ret = append(ret, f)
	}
	h.log(log.TraceLevel, "record add workspace: %v", ret)
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].UpdatedAt.Before(ret[j].UpdatedAt)
	})
	return ret
}

func (h *history) recordCloseWorkspace(uri workspaceapi.URI) {
	// no need to unsubscibe as everything will be garbage collected
	// and workspaceHistory has protection against receiving events
	// once already deleted.
	delete(h.cache, uri.String())
}

func (h *history) dirtyFilesOpen() (ret bool) {
	for _, w := range h.cache {
		for _, f := range w.Files {
			if f.Dirty {
				return true
			}
		}
	}
	return false
}

func (h *history) resetWorkspaceCache(uri workspaceapi.URI) {
	const cacheSetTimeout = 1 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), cacheSetTimeout)
	defer cancel()

	fresh := cache{Files: make(map[string]file)}
	h.cache[uri.String()] = fresh

	err := h.svc.Set(ctx, uri.String(), fresh)
	if err != nil {
		h.log(log.WarnLevel, "could not persist new cache to durable storage: %v", err)
		return
	}
	h.log(log.TraceLevel, "persisted new cache for workspace %s", uri.String())
}

type workspaceHistory struct {
	uri   string
	cache map[string]cache
	svc   document.Service
}

func (h *workspaceHistory) Handle(ctx context.Context, ev textapi.Event) bool {
	workspaceCache, ok := h.cache[h.uri]
	if !ok {
		return true // we're done if workspace was deleted
	}

	// do not assume dispatch of events is correct
	if ev.URI == (workspaceapi.URI{}) {
		return false
	}

	evUriStr := ev.URI.String()

	switch ev.Type {
	case textapi.EventTypeOpen:
		workspaceCache.Files[evUriStr] = makeFile(evUriStr, false)
	case textapi.EventTypeClose:
		delete(workspaceCache.Files, evUriStr)
	case textapi.EventTypeFlush:
		workspaceCache.Files[evUriStr] = makeFile(evUriStr, false)
	case textapi.EventTypeEdit:
		workspaceCache.Files[evUriStr] = makeFile(evUriStr, true)
	}

	h.persistUpdateCache(h.uri, workspaceCache)
	return false
}

func makeFile(uri string, dirty bool) file {
	return file{Dirty: dirty, URIString: uri, UpdatedAt: time.Now()}
}

func (h *workspaceHistory) persistUpdateCache(uri string, cache cache) {
	const cacheSetTimeout = 1 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), cacheSetTimeout)
	defer cancel()

	err := h.svc.Set(ctx, uri, cache)
	if err != nil {
		log.WithFields(log.Fields{logging.KeyClass: "ide.history"}).
			Warnf("could not persist updated cache to durable storage: %v", err)
		return
	}
	log.WithFields(log.Fields{logging.KeyClass: "ide.history"}).
		Tracef("persisted updated cache for workspace %s", h.uri)
}
