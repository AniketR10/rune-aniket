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

package localstorage

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"unstable.build/rune/debug"
	"unstable.build/rune/localstorage/boltdoc"
	"unstable.build/rune/localstorage/firstmover"
)

// New returns a storageapi.Service storage service that uses the local
// directory dir to setup a local, filesystem-backed, multi-process safe,
// goroutine-safe storageapi.Service.
func New(_ context.Context, dir string, marshaler docmarshal.Marshaler) storageapi.Service {
	ret := &delayedLoadingService{ready: make(chan struct{})}
	go debug.CapturePanicReport(func() {
		defer close(ret.ready)

		storageDir := filepath.Join(dir, "run")
		err := os.MkdirAll(storageDir, 0777)
		if err != nil {
			log.Errorf("new storage: mkdir: %v", err)
			ret.service = storagestub.NewInMemoryService()
			return
		}
		dbPath := filepath.Join(storageDir, "db.data")
		cfg := firstmover.DefaultConfig()
		cfg.Marshaler = marshaler
		cfg.CloseError = boltdoc.ErrClosing

		open := func() (storageapi.Service, error) {
			return boltdoc.New(dbPath, marshaler)
		}
		ret.service = firstmover.New(open, filepath.Join(dir, "db.lock"), cfg)
	})
	return ret
}

type delayedLoadingService struct {
	service storageapi.Service
	ready   chan struct{}
}

func (d *delayedLoadingService) waitReady(ctx context.Context) error {
	if d.ready == nil {
		return nil
	}
	select {
	case <-d.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *delayedLoadingService) Create(ctx context.Context, ID string, doc any) error {
	if err := d.waitReady(ctx); err != nil {
		return err
	}
	return d.service.Create(ctx, ID, doc)
}

func (d *delayedLoadingService) Set(ctx context.Context, ID string, doc any) error {
	if err := d.waitReady(ctx); err != nil {
		return err
	}
	return d.service.Set(ctx, ID, doc)
}

func (d *delayedLoadingService) Update(ctx context.Context, ID string,
	updates []storageapi.Update, precond ...storageapi.Precondition) error {
	if err := d.waitReady(ctx); err != nil {
		return err
	}
	return d.service.Update(ctx, ID, updates, precond...)
}

func (d *delayedLoadingService) Get(ctx context.Context, ID string, doc any) error {
	if err := d.waitReady(ctx); err != nil {
		return err
	}
	return d.service.Get(ctx, ID, doc)
}

func (d *delayedLoadingService) Delete(ctx context.Context, ID string) error {
	if err := d.waitReady(ctx); err != nil {
		return err
	}
	return d.service.Delete(ctx, ID)
}

// Drop satisfies storageapi.DroppableService.
func (d *delayedLoadingService) Drop(ctx context.Context) error {
	if err := d.waitReady(ctx); err != nil {
		return err
	}
	droppable, ok := d.service.(storageapi.DroppableService)
	if !ok {
		return errors.New("storage service does not support dropping")
	}
	return droppable.Drop(ctx)
}

// ApplyBatch satisfies storageapi.BatchWriter.
func (d *delayedLoadingService) ApplyBatch(
	ctx context.Context, ops []storageapi.BatchOp,
) ([]storageapi.BatchOpResult, error) {
	if err := d.waitReady(ctx); err != nil {
		return nil, err
	}
	writer, ok := d.service.(storageapi.BatchWriter)
	if !ok {
		return nil, errors.New("storage service does not support batching")
	}
	return writer.ApplyBatch(ctx, ops)
}

func (d *delayedLoadingService) List(ctx context.Context, filters []storageapi.Filter) (
	storageapi.Iterator, error,
) {
	if err := d.waitReady(ctx); err != nil {
		return nil, err
	}
	return d.service.List(ctx, filters)
}

func (d *delayedLoadingService) Partition(name string) (storageapi.Service, error) {
	if d.ready != nil {
		<-d.ready
	}
	return d.service.Partition(name)
}

func (d *delayedLoadingService) Close() error {
	if d.ready != nil {
		<-d.ready
	}
	return d.service.Close()
}
