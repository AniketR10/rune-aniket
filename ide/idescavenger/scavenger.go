// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package idescavenger

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
)

const (
	// partitionName is the sub-partition the Cleaner owns. It holds
	// nothing but the tracking document.
	partitionName = "scavenger"

	trackedDocumentID   = "workspaces"
	trackedDocumentKind = "scavenger-workspaces"

	fileScheme = "file"
)

// WorkspaceHook reclaims the storage a workspace left behind. Hooks are
// only invoked for workspaces whose root directory no longer exists, and
// must be idempotent: a hook that fails is retried on the next pass.
type WorkspaceHook func(ctx context.Context, cwd workspaceapi.URI) error

// Config configures a Cleaner.
type Config struct {
	// Storage is the IDE storage. The Cleaner partitions its own
	// dataset out of it and does not take ownership of it.
	Storage storageapi.Service

	// OpenWorkspaces returns the workspaces that are currently open. A
	// pass that cannot determine them is abandoned rather than risking
	// the storage of a live workspace. Optional: when nil, no workspace
	// is considered open.
	OpenWorkspaces func(ctx context.Context) ([]workspaceapi.URI, error)

	// Stat resolves a workspace root. Defaults to os.Stat.
	Stat func(name string) (fs.FileInfo, error)
}

// Cleaner reclaims the storage of workspaces that no longer exist on
// disk. It is safe for concurrent use.
type Cleaner struct {
	storage        storageapi.Service
	openWorkspaces func(ctx context.Context) ([]workspaceapi.URI, error)
	stat           func(name string) (fs.FileInfo, error)

	mu    sync.Mutex
	hooks []WorkspaceHook
}

// New returns a Cleaner that tracks workspaces in its own partition of
// cfg.Storage.
func New(cfg Config) (*Cleaner, error) {
	if cfg.Storage == nil {
		return nil, errors.New("idescavenger: storage is required")
	}
	storage, err := cfg.Storage.Partition(partitionName)
	if err != nil {
		return nil, fmt.Errorf("idescavenger: partition storage: %w", err)
	}
	stat := cfg.Stat
	if stat == nil {
		stat = os.Stat
	}
	return &Cleaner{
		storage:        storage,
		openWorkspaces: cfg.OpenWorkspaces,
		stat:           stat,
	}, nil
}

// AddWorkspaceHook registers a hook run against every workspace whose
// root has disappeared. A workspace stops being tracked once all of its
// hooks have succeeded, so hooks must be registered before Start.
func (c *Cleaner) AddWorkspaceHook(hook WorkspaceHook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hooks = append(c.hooks, hook)
}

// RegisterNewWorkspace starts tracking uri. It is idempotent.
func (c *Cleaner) RegisterNewWorkspace(
	ctx context.Context, uri workspaceapi.URI,
) error {
	return c.register(ctx, uri)
}

// Seed starts tracking workspaces that predate the Cleaner, typically
// recovered from another IDE dataset.
func (c *Cleaner) Seed(
	ctx context.Context, uris []workspaceapi.URI,
) error {
	return c.register(ctx, uris...)
}

// Start runs a single pass in the background.
func (c *Cleaner) Start(ctx context.Context) {
	go debug.CapturePanicReport(func() {
		if err := c.RunOnce(ctx); err != nil {
			logger().Errorf("scavenge workspaces: %v", err)
		}
	})
}

// RunOnce drops the storage of every tracked workspace whose root
// directory no longer exists.
func (c *Cleaner) RunOnce(ctx context.Context) error {
	tracked, err := c.load(ctx)
	if err != nil {
		return err
	}
	if len(tracked.URIs) == 0 {
		return nil
	}

	open, err := c.openWorkspaceSet(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	hooks := append([]WorkspaceHook(nil), c.hooks...)
	c.mu.Unlock()

	remaining := make([]string, 0, len(tracked.URIs))
	for _, raw := range tracked.URIs {
		keep, err := c.scavenge(ctx, hooks, open, raw)
		if err != nil {
			return err
		}
		if keep {
			remaining = append(remaining, raw)
		}
	}
	if len(remaining) == len(tracked.URIs) {
		return nil
	}
	return c.store(ctx, remaining)
}

// scavenge reports whether raw must stay tracked.
func (c *Cleaner) scavenge(
	ctx context.Context, hooks []WorkspaceHook,
	open map[string]struct{}, raw string,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, err
	}
	uri, err := workspaceapi.ParseURI(raw)
	if err != nil {
		// unparseable entries can never be acted upon
		return false, nil
	}
	if _, isOpen := open[raw]; isOpen {
		return true, nil
	}
	// only local roots can be told apart from an unreachable host
	if uri.Scheme() != fileScheme {
		return true, nil
	}
	// a stat error other than "not there" (a permission problem, an
	// unmounted volume) says nothing about the workspace being gone
	if _, err := c.stat(uri.Path()); !errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}

	for _, hook := range hooks {
		if err := hook(ctx, uri); err != nil {
			logger().Errorf("scavenge workspace %q: %v", raw, err)
			return true, nil
		}
	}
	logger().Debugf("scavenged storage of removed workspace %q", raw)
	return false, nil
}

func (c *Cleaner) openWorkspaceSet(
	ctx context.Context,
) (map[string]struct{}, error) {
	if c.openWorkspaces == nil {
		return nil, nil
	}
	uris, err := c.openWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("idescavenger: open workspaces: %w", err)
	}
	open := make(map[string]struct{}, len(uris))
	for _, uri := range uris {
		open[uri.String()] = struct{}{}
	}
	return open, nil
}

// register adds uris to the tracking document. The read-modify-write is
// only guarded within this process: a registration lost to another IDE
// process is recovered the next time that workspace is opened.
func (c *Cleaner) register(
	ctx context.Context, uris ...workspaceapi.URI,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tracked, err := c.load(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]struct{}, len(tracked.URIs))
	for _, raw := range tracked.URIs {
		known[raw] = struct{}{}
	}

	added := false
	for _, uri := range uris {
		// remote roots cannot be distinguished from an unreachable
		// host, so they are never scavenged and never tracked
		if uri.Scheme() != fileScheme {
			continue
		}
		raw := uri.String()
		if _, ok := known[raw]; ok {
			continue
		}
		known[raw] = struct{}{}
		tracked.URIs = append(tracked.URIs, raw)
		added = true
	}
	if !added {
		return nil
	}
	return c.store(ctx, tracked.URIs)
}

type trackedWorkspaces struct {
	Kind string
	URIs []string
}

func (c *Cleaner) load(ctx context.Context) (trackedWorkspaces, error) {
	var doc trackedWorkspaces
	err := c.storage.Get(ctx, trackedDocumentID, &doc)
	if errors.Is(err, storageapi.ErrNotFound) {
		return trackedWorkspaces{}, nil
	}
	if err != nil {
		return trackedWorkspaces{}, fmt.Errorf(
			"idescavenger: load tracked workspaces: %w", err)
	}
	return doc, nil
}

func (c *Cleaner) store(ctx context.Context, uris []string) error {
	doc := trackedWorkspaces{Kind: trackedDocumentKind, URIs: uris}
	if err := c.storage.Set(ctx, trackedDocumentID, &doc); err != nil {
		return fmt.Errorf("idescavenger: store tracked workspaces: %w", err)
	}
	return nil
}

// Close releases the partition handle owned by this Cleaner.
func (c *Cleaner) Close() error {
	return c.storage.Close()
}

func logger() *log.Entry {
	return log.WithField(logging.KeyClass, "ide.idescavenger")
}
