// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package configedit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"gopkg.in/yaml.v3"
)

// FS is the minimal filesystem surface needed to read and write
// .rune/config.yaml.
type FS interface {
	OpenFile(path string, flag int, perm os.FileMode) (workspaceapi.File, error)
	MkdirAll(path string, perm os.FileMode) error
}

// NewConfig builds a Config that reads from snapshot and persists
// writes to <cwd>/.rune/config.yaml via fs. snapshot is the host
// extension configuration provided by the SDK; it is consulted as a
// read-through fallback when a key is not in the in-memory overlay.
//
// fs must not be nil. snapshot may be nil; in that case Get* values
// resolve only through the overlay.
func NewConfig(fs FS, cwd workspaceapi.URI, snapshot config.Config) Config {
	if fs == nil {
		panic("configedit: fs must not be nil")
	}
	return &editor{
		fs:       fs,
		cwd:      cwd,
		snapshot: snapshot,
		overlay:  make(map[string]any),
	}
}

// editor is the concrete Config implementation. Reads consult the
// overlay first, then snapshot; writes update both the overlay and
// .rune/config.yaml under a mutex so concurrent agent calls observe
// consistent state.
type editor struct {
	fs       FS
	cwd      workspaceapi.URI
	snapshot config.Config

	mu      sync.RWMutex
	overlay map[string]any
}

// --- Getter implementation --------------------------------------------------

func (e *editor) GetBool(key string) Bool {
	return Bool{resolve: func(context.Context) (bool, error) {
		e.mu.RLock()
		if v, ok := e.overlay[key]; ok {
			e.mu.RUnlock()
			b, ok := v.(bool)
			if !ok {
				return false, ErrInvalidType
			}
			return b, nil
		}
		e.mu.RUnlock()
		if e.snapshot == nil {
			return false, ErrNotFound
		}
		v, err := e.snapshot.GetBool(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetInt(key string) Int {
	return Int{resolve: func(context.Context) (int, error) {
		e.mu.RLock()
		if v, ok := e.overlay[key]; ok {
			e.mu.RUnlock()
			switch vt := v.(type) {
			case int:
				return vt, nil
			case int64:
				return int(vt), nil
			case float64:
				return int(vt), nil
			default:
				return 0, ErrInvalidType
			}
		}
		e.mu.RUnlock()
		if e.snapshot == nil {
			return 0, ErrNotFound
		}
		v, err := e.snapshot.GetInt(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetFloat(key string) Float {
	return Float{resolve: func(context.Context) (float64, error) {
		e.mu.RLock()
		if v, ok := e.overlay[key]; ok {
			e.mu.RUnlock()
			switch vt := v.(type) {
			case float64:
				return vt, nil
			case int:
				return float64(vt), nil
			default:
				return 0, ErrInvalidType
			}
		}
		e.mu.RUnlock()
		if e.snapshot == nil {
			return 0, ErrNotFound
		}
		v, err := e.snapshot.GetFloat(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetString(key string) String {
	return String{resolve: func(context.Context) (string, error) {
		e.mu.RLock()
		if v, ok := e.overlay[key]; ok {
			e.mu.RUnlock()
			s, ok := v.(string)
			if !ok {
				return "", ErrInvalidType
			}
			return s, nil
		}
		e.mu.RUnlock()
		if e.snapshot == nil {
			return "", ErrNotFound
		}
		v, err := e.snapshot.GetString(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetMap(key string) Map {
	return Map{resolve: func(context.Context) (map[string]any, error) {
		e.mu.RLock()
		if v, ok := e.overlay[key]; ok {
			e.mu.RUnlock()
			m, ok := v.(map[string]any)
			if !ok {
				return nil, ErrInvalidType
			}
			return m, nil
		}
		e.mu.RUnlock()
		if e.snapshot == nil {
			return nil, ErrNotFound
		}
		v, err := e.snapshot.GetMap(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetSlice(key string) Slice {
	return Slice{resolve: func(context.Context) ([]any, error) {
		e.mu.RLock()
		if v, ok := e.overlay[key]; ok {
			e.mu.RUnlock()
			s, ok := v.([]any)
			if !ok {
				return nil, ErrInvalidType
			}
			return s, nil
		}
		e.mu.RUnlock()
		if e.snapshot == nil {
			return nil, ErrNotFound
		}
		v, err := e.snapshot.GetSlice(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetRune(key string) Rune {
	return Rune{resolve: func(context.Context) (rune, error) {
		e.mu.RLock()
		if v, ok := e.overlay[key]; ok {
			e.mu.RUnlock()
			switch vt := v.(type) {
			case rune:
				return vt, nil
			case string:
				if vt == "" {
					return 0, ErrInvalidType
				}
				return []rune(vt)[0], nil
			default:
				return 0, ErrInvalidType
			}
		}
		e.mu.RUnlock()
		if e.snapshot == nil {
			return 0, ErrNotFound
		}
		v, err := e.snapshot.GetRune(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetColor(key string) Color {
	return Color{resolve: func(context.Context) (term.Color, error) {
		// Overlay does not currently carry colors. Defer to snapshot.
		if e.snapshot == nil {
			return 0, ErrNotFound
		}
		v, err := e.snapshot.GetColor(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetAttribute(key string) Attribute {
	return Attribute{resolve: func(context.Context) (term.AttrMask, error) {
		if e.snapshot == nil {
			return 0, ErrNotFound
		}
		v, err := e.snapshot.GetAttribute(key)
		return v, translateErr(err)
	}}
}

func (e *editor) GetConfig(key string) ConfigValue {
	return ConfigValue{resolve: func(context.Context) (Getter, error) {
		if e.snapshot == nil {
			return nil, ErrNotFound
		}
		sub, err := e.snapshot.GetConfig(key)
		if err != nil {
			return nil, translateErr(err)
		}
		// Nested configs are read-only views over the snapshot — the
		// overlay only applies to top-level keys. Wrap the SDK Config
		// in a thin adapter so callers see configedit value types.
		return &snapshotGetter{snapshot: sub}, nil
	}}
}

func (e *editor) Iterate(fn func(key string, value any)) {
	// Iterate the overlay first, then any snapshot keys not shadowed.
	seen := make(map[string]struct{})
	e.mu.RLock()
	for k, v := range e.overlay {
		fn(k, v)
		seen[k] = struct{}{}
	}
	e.mu.RUnlock()
	if e.snapshot == nil {
		return
	}
	e.snapshot.Iterate(func(k string, v any) {
		if _, ok := seen[k]; ok {
			return
		}
		fn(k, v)
	})
}

// --- Setter implementation --------------------------------------------------

func (e *editor) SetBool(_ context.Context, key string, v, ephemeral bool) error {
	return e.setScalar(key, v, "!!bool", strconv.FormatBool(v), ephemeral)
}

func (e *editor) SetInt(_ context.Context, key string, v int, ephemeral bool) error {
	return e.setScalar(key, v, "!!int", strconv.Itoa(v), ephemeral)
}

func (e *editor) SetFloat(_ context.Context, key string, v float64, ephemeral bool) error {
	return e.setScalar(key, v, "!!float", strconv.FormatFloat(v, 'g', -1, 64), ephemeral)
}

func (e *editor) SetString(_ context.Context, key string, v string, ephemeral bool) error {
	return e.setScalar(key, v, "!!str", v, ephemeral)
}

func (e *editor) AppendStringSlice(_ context.Context, key, v string, ephemeral bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	current, _ := e.overlayOrSnapshotSlice(key)
	for _, existing := range current {
		if s, ok := existing.(string); ok && s == v {
			return ErrAlreadyPresent
		}
	}
	updated := append(append([]any(nil), current...), v)
	if !ephemeral {
		if err := e.persistSequence(key, updated); err != nil {
			return err
		}
	}
	e.overlay[key] = updated
	return nil
}

func (e *editor) RemoveStringSlice(_ context.Context, key, v string, ephemeral bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	current, _ := e.overlayOrSnapshotSlice(key)
	idx := -1
	for i, existing := range current {
		if s, ok := existing.(string); ok && s == v {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotPresent
	}
	updated := append(append([]any(nil), current[:idx]...), current[idx+1:]...)
	if !ephemeral {
		if err := e.persistSequence(key, updated); err != nil {
			return err
		}
	}
	e.overlay[key] = updated
	return nil
}

func (e *editor) setScalar(key string, overlayValue any, tag, encoded string, ephemeral bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !ephemeral {
		if err := e.persistScalar(key, tag, encoded); err != nil {
			return err
		}
	}
	e.overlay[key] = overlayValue
	return nil
}

// overlayOrSnapshotSlice returns the current sequence value for key
// (overlay first, snapshot second), or nil if absent. Caller must hold
// e.mu.
func (e *editor) overlayOrSnapshotSlice(key string) ([]any, bool) {
	if v, ok := e.overlay[key]; ok {
		if s, ok := v.([]any); ok {
			return s, true
		}
	}
	if e.snapshot == nil {
		return nil, false
	}
	s, err := e.snapshot.GetSlice(key)
	if err != nil {
		return nil, false
	}
	return s, true
}

// --- YAML persistence -------------------------------------------------------

func (e *editor) configFilePath() string {
	return workspaceapi.Join(e.cwd, ".rune", "config.yaml").Path()
}

func (e *editor) persistScalar(key, tag, encoded string) error {
	path := e.configFilePath()
	root, err := readConfigNode(e.fs, path)
	if err != nil {
		return err
	}
	cfg := findOrCreateRuneAgentConfig(root)
	setScalarKey(cfg, key, encoded, tag)
	return writeConfigNode(e.fs, path, root)
}

func (e *editor) persistSequence(key string, values []any) error {
	path := e.configFilePath()
	root, err := readConfigNode(e.fs, path)
	if err != nil {
		return err
	}
	cfg := findOrCreateRuneAgentConfig(root)
	setStringSeqKey(cfg, key, values)
	return writeConfigNode(e.fs, path, root)
}

// translateErr maps the SDK config errors onto the configedit ones so
// callers can use errors.Is(err, configedit.ErrNotFound) uniformly.
func translateErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, config.ErrNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, config.ErrInvalidType) {
		return ErrInvalidType
	}
	return err
}

// snapshotGetter adapts a SDK config.Config to configedit.Getter.
// Used for nested configs (GetConfig); writes are not supported on
// nested views because the overlay is keyed at the top level only.
type snapshotGetter struct {
	snapshot config.Config
}

func (s *snapshotGetter) GetBool(key string) Bool {
	return Bool{resolve: func(context.Context) (bool, error) {
		v, err := s.snapshot.GetBool(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetInt(key string) Int {
	return Int{resolve: func(context.Context) (int, error) {
		v, err := s.snapshot.GetInt(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetFloat(key string) Float {
	return Float{resolve: func(context.Context) (float64, error) {
		v, err := s.snapshot.GetFloat(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetString(key string) String {
	return String{resolve: func(context.Context) (string, error) {
		v, err := s.snapshot.GetString(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetMap(key string) Map {
	return Map{resolve: func(context.Context) (map[string]any, error) {
		v, err := s.snapshot.GetMap(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetSlice(key string) Slice {
	return Slice{resolve: func(context.Context) ([]any, error) {
		v, err := s.snapshot.GetSlice(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetRune(key string) Rune {
	return Rune{resolve: func(context.Context) (rune, error) {
		v, err := s.snapshot.GetRune(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetColor(key string) Color {
	return Color{resolve: func(context.Context) (term.Color, error) {
		v, err := s.snapshot.GetColor(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetAttribute(key string) Attribute {
	return Attribute{resolve: func(context.Context) (term.AttrMask, error) {
		v, err := s.snapshot.GetAttribute(key)
		return v, translateErr(err)
	}}
}

func (s *snapshotGetter) GetConfig(key string) ConfigValue {
	return ConfigValue{resolve: func(context.Context) (Getter, error) {
		sub, err := s.snapshot.GetConfig(key)
		if err != nil {
			return nil, translateErr(err)
		}
		return &snapshotGetter{snapshot: sub}, nil
	}}
}

func (s *snapshotGetter) Iterate(fn func(string, any)) {
	s.snapshot.Iterate(fn)
}

// --- low-level YAML helpers -------------------------------------------------

func readConfigNode(fs FS, path string) (*yaml.Node, error) {
	f, err := fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		return emptyDocNode(), nil
	}
	defer f.Close() //nolint:errcheck

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if doc.Kind == 0 {
		return emptyDocNode(), nil
	}
	return &doc, nil
}

func emptyDocNode() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		},
	}
}

func writeConfigNode(fs FS, path string, doc *yaml.Node) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("close encoder: %w", err)
	}
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	wf, err := fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	defer wf.Close() //nolint:errcheck
	if _, err := wf.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func findOrCreateRuneAgentConfig(doc *yaml.Node) *yaml.Node {
	root := doc.Content[0]
	extensions := findOrCreateMapKey(root, "extensions")
	runeAgent := findOrCreateMapKey(extensions, "rune-agent")
	return findOrCreateMapKey(runeAgent, "config")
}

func findOrCreateMapKey(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content, keyNode, valNode)
	return valNode
}

func setScalarKey(parent *yaml.Node, key, value, tag string) {
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
	parent.Content = append(parent.Content, keyNode, valNode)
}

func setStringSeqKey(parent *yaml.Node, key string, values []any) {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, v := range values {
		s, _ := v.(string)
		seq.Content = append(seq.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!str", Value: s,
		})
	}
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1] = seq
			return
		}
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, seq)
}
