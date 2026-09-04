// Copyright (C) 2017-2026 Unstable Build, LLC
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

package schemedoc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"

	multierr "github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// ErrClosing is returned to inflight write requests
// before comitting changes to disk if the service
// is currently closing.
var ErrClosing = errors.New("service is closing")

// NewDocumentService returns a storageapi.Service backed by a schemeapi.Scheme.
// It its goroutine-safe but only one instance can be operating at a time
// on a given workspace.
func NewDocumentService(scheme schemeapi.Scheme, marshaler docmarshal.Marshaler) (
	storageapi.Service, error,
) {
	svc := service{
		scheme:    scheme,
		marshaler: marshaler,
	}
	// make it goroutine-safe
	return Sync(&svc), nil
}

type service struct {
	scheme    schemeapi.Scheme
	marshaler docmarshal.Marshaler

	// Used to wait on all writes before Close returns.
	// This is to guarantee that once lock is released,
	// there are no writes to the underlying files.
	wg closeGroup
}

func (s *service) Create(ctx context.Context, ID string, doc any) error {
	return s.create(ctx, ID, doc, os.O_CREATE|os.O_EXCL|os.O_WRONLY)
}

func (s *service) Set(ctx context.Context, ID string, doc any) error {
	return s.create(ctx, ID, doc, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
}

func (s *service) Update(
	ctx context.Context, ID string, updates []storageapi.Update,
	preconds ...storageapi.Precondition,
) error {
	if len(updates) == 0 {
		panic("Update: no paths to update")
	}
	if ID == "" {
		return errors.New("invalid ID: empty")
	}
	origFileName := s.getFileName(ID)
	orig, err := s.scheme.OpenFile(origFileName, os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storageapi.ErrNotFound
		}
		if errors.Is(err, os.ErrPermission) {
			return storageapi.ErrPermissionDenied
		}
		return fmt.Errorf("scheme open: %w", err)
	}

	proto := make(map[string]any)
	err = s.read(orig, &proto)
	if cerr := orig.Close(); cerr != nil {
		err = multierr.Append(err, cerr)
	}
	if err != nil {
		return err
	}

	// order of operations (open .swp, update proto, etc) doesn't matter
	// because all methods are serialized via Sync, and
	// there can only be one instance of this storageapi.Service operating
	// at a given workspace at a time.
	err = storageapi.UpdateProto(s.marshaler, updates, proto, preconds...)
	if err != nil {
		return err
	}

	targetFileName := s.getFileName(ID) + ".swp"
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	target, err := s.scheme.OpenFile(targetFileName, flag, 0666)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return storageapi.ErrPermissionDenied
		}
		return fmt.Errorf("scheme open: %w", err)
	}

	err = s.write(target, proto)
	if cerr := target.Close(); cerr != nil {
		err = multierr.Append(err, cerr)
	}
	if err != nil {
		return err
	}

	err = s.scheme.Rename(targetFileName, origFileName)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return storageapi.ErrPermissionDenied
		}
		return fmt.Errorf("scheme rename: %v", err)
	}

	return nil
}

func (s *service) Get(ctx context.Context, ID string, doc any) error {
	if ID == "" {
		return errors.New("invalid ID: empty")
	}
	if !storageapi.IsEncodeable(doc) {
		return errors.New("invalid document argument")
	}
	f, err := s.scheme.OpenFile(s.getFileName(ID), os.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storageapi.ErrNotFound
		}
		if errors.Is(err, os.ErrPermission) {
			return storageapi.ErrPermissionDenied
		}
		return fmt.Errorf("scheme open: %w", err)
	}
	defer f.Close()

	return s.read(f, doc)
}

func (s *service) Delete(ctx context.Context, ID string) error {
	if ID == "" {
		return errors.New("invalid ID: empty")
	}
	err := s.scheme.Remove(s.getFileName(ID))
	// delete should be idempotent
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if errors.Is(err, os.ErrPermission) {
		return storageapi.ErrPermissionDenied
	}
	return err
}

func (s *service) List(
	ctx context.Context, filters []storageapi.Filter,
) (storageapi.Iterator, error) {
	for _, f := range filters {
		if len(f.FieldPath) == 0 || f.Op == "" {
			panic("invalid filter")
		}
	}
	entries, err := s.scheme.ReadDir(".")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, storageapi.ErrPermissionDenied
		}
		return nil, fmt.Errorf("scheme list files: %v", err)
	}
	it := iterator.Map(iterator.FromSlice(entries), func(entry os.DirEntry) string {
		return entry.Name()
	})
	return &docIter{ctx: ctx, filters: filters, svc: s, it: it}, nil
}

func (s *service) Partition(name string) (storageapi.Service, error) {
	if name == "" {
		return nil, errors.New("invalid partition: empty")
	}
	partitionDir := partitionDirName(name)
	if err := s.scheme.MkdirAll(partitionDir, 0777); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, storageapi.ErrPermissionDenied
		}
		return nil, fmt.Errorf("mkdir partition %q: %w", name, err)
	}
	partitionScheme, err := s.scheme.Chroot(partitionDir)
	if err != nil {
		return nil, fmt.Errorf("chroot partition %q: %w", name, err)
	}
	return Sync(&service{scheme: partitionScheme, marshaler: s.marshaler}), nil
}

func (s *service) Close() (ret error) {
	// wait on writes to finish and prevent any new
	// writes from making progress
	if !s.wg.Close() {
		// already closed
		return
	}
	if err := s.scheme.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

func (s *service) read(f workspaceapi.File, doc any) error {
	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("file read all: %v", err)
	}

	err = s.marshaler.Unmarshal(data, doc)
	if err != nil {
		return fmt.Errorf("unmarshal %s: %v: %s", f.Name(), err, string(data))
	}
	return nil
}

func (s *service) getFileName(id string) string {
	return url.PathEscape(id)
}

func partitionDirName(name string) string {
	return url.PathEscape(name)
}

func (s *service) create(ctx context.Context, ID string, doc any, openFlags int) error {
	if doc == nil {
		panic("invalid nil data argument to Create/Set")
	}
	if ID == "" {
		return errors.New("invalid ID: empty")
	}
	doc, err := storageapi.DerefCreateValue(reflect.ValueOf(doc))
	if err != nil {
		return err
	}
	f, err := s.scheme.OpenFile(s.getFileName(ID), openFlags, 0666)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return storageapi.ErrAlreadyExists
		}
		if errors.Is(err, os.ErrPermission) {
			return storageapi.ErrPermissionDenied
		}
		return fmt.Errorf("scheme open: %w", err)
	}

	doc = storageapi.UpdateCreatedAtField(s.marshaler, doc)

	var ret error
	if err := s.write(f, doc); err != nil {
		ret = multierr.Append(ret, err)
	}
	if cerr := f.Close(); cerr != nil {
		ret = multierr.Append(ret, cerr)
	}
	return ret
}

func (s *service) write(f workspaceapi.File, doc any) error {
	data, err := s.marshaler.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal: %v", err)
	}
	done, ok := s.wg.AddOne()
	if !ok {
		return ErrClosing
	}
	defer done()
	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("file write: %v", err)
	}
	err = f.Sync()
	if err != nil {
		return fmt.Errorf("file sync: %v", err)
	}
	return nil
}

type docIter struct {
	ctx     context.Context
	filters []storageapi.Filter
	svc     *service
	it      iterator.Iterator[string]

	doneErr       error
	nextMatchFile workspaceapi.File
}

func (d *docIter) HasNext() (ok bool) {
	for d.nextMatchFile == nil && d.doneErr == nil {
		nextFile, ok := d.it.Next(d.ctx)
		if !ok {
			d.doneErr = d.it.Err()
			return false
		}
		f, err := d.svc.scheme.OpenFile(d.svc.getFileName(nextFile), os.O_RDONLY, 0)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // should not happend but let's be resilient
			}
			if errors.Is(err, os.ErrPermission) {
				d.doneErr = storageapi.ErrPermissionDenied
				return true
			}
			d.doneErr = err
			return true
		}

		proto := make(map[string]any)
		err = d.svc.read(f, &proto)
		if err != nil {
			if cerr := f.Close(); cerr != nil {
				d.doneErr = cerr
				return true
			}
			continue
		}

		if storageapi.MatchesAllFilters(d.svc.marshaler, proto, d.filters) {
			d.nextMatchFile = f
			return true
		}

		if cerr := f.Close(); cerr != nil {
			d.doneErr = cerr
			return true
		}

		/* continue */
	}

	return d.doneErr == nil
}

func (d *docIter) NextTo(doc any) error {
	if !storageapi.IsEncodeable(doc) {
		return errors.New("receiver is not a pointer and not a map or is nil")
	}
	if d.doneErr != nil {
		return d.doneErr
	}
	if d.nextMatchFile == nil {
		if !d.HasNext() {
			return errors.New("exhausted iterator")
		}
		if d.doneErr != nil {
			return d.doneErr
		}
	}
	f := d.nextMatchFile
	d.nextMatchFile = nil
	defer f.Close()
	_, err := f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("file seek: %v", err)
	}
	return d.svc.read(f, doc)
}

func (d *docIter) Close() error {
	return d.it.Close()
}
