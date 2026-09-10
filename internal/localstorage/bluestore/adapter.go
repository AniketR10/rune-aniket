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

package bluestore

import (
	"context"
	"errors"
	"fmt"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

// AdaptTo wraps a document.Service to satisfy storageapi.Service.
func AdaptTo(svc document.Service) storageapi.Service {
	return adapter{svc}
}

// AdaptFrom wraps a storageapi.Service to satisfy document.Service.
func AdaptFrom(svc storageapi.Service) document.Service {
	return wrap{svc}
}

type adapter struct {
	document.Service
}

func (a adapter) Create(ctx context.Context, ID string, doc any) error {
	err := a.Service.Create(ctx, ID, doc)
	switch {
	case errors.Is(err, document.ErrAlreadyExists):
		return storageapi.ErrAlreadyExists
	case errors.Is(err, document.ErrPermissionDenied):
		return storageapi.ErrPermissionDenied
	case errors.Is(err, document.ErrNotFound):
		return storageapi.ErrNotFound
	}
	return err
}

func (a adapter) Set(ctx context.Context, ID string, doc any) error {
	err := a.Service.Set(ctx, ID, doc)
	switch {
	case errors.Is(err, document.ErrPermissionDenied):
		return storageapi.ErrPermissionDenied
	case errors.Is(err, document.ErrNotFound):
		return storageapi.ErrNotFound
	}
	return err
}

func (a adapter) Get(ctx context.Context, ID string, doc any) error {
	err := a.Service.Get(ctx, ID, doc)
	switch {
	case errors.Is(err, document.ErrPermissionDenied):
		return storageapi.ErrPermissionDenied
	case errors.Is(err, document.ErrNotFound):
		return storageapi.ErrNotFound
	}
	return err
}

func (a adapter) Delete(ctx context.Context, ID string) error {
	err := a.Service.Delete(ctx, ID)
	switch {
	case errors.Is(err, document.ErrPermissionDenied):
		return storageapi.ErrPermissionDenied
	}
	return err
}

func (a adapter) List(ctx context.Context, filters []storageapi.Filter) (
	storageapi.Iterator, error,
) {
	var svcFilters []document.Filter
	for _, filter := range filters {
		svcFilters = append(svcFilters, document.Filter{
			Field: document.Field{
				FieldPath: filter.FieldPath,
				Value:     filter.Value,
			},
			Op: document.Op(filter.Op),
		})
	}
	it, err := a.Service.List(ctx, svcFilters)
	if err == nil {
		return it, err
	}
	switch {
	case errors.Is(err, document.ErrPermissionDenied):
		return nil, storageapi.ErrPermissionDenied
	}
	return it, err
}

func (a adapter) Update(
	ctx context.Context, ID string,
	updates []storageapi.Update, precond ...storageapi.Precondition,
) error {
	var svcUpdates []document.Update
	for _, filter := range updates {
		svcUpdates = append(svcUpdates, document.Update{
			FieldPath: filter.FieldPath,
			Value:     filter.Value,
		})
	}
	var svcPreconds []document.Precondition
	for _, filter := range precond {
		svcPreconds = append(svcPreconds, document.Precondition{
			FieldPath: filter.FieldPath,
			Value:     filter.Value,
		})
	}
	err := a.Service.Update(ctx, ID, svcUpdates, svcPreconds...)
	switch {
	case errors.Is(err, document.ErrPermissionDenied):
		return storageapi.ErrPermissionDenied
	case errors.Is(err, document.ErrNotFound):
		return storageapi.ErrNotFound
	case errors.Is(err, document.ErrPreconditionFailed):
		return storageapi.ErrPreconditionFailed
	}
	return err
}

func (a adapter) Partition(name string) (storageapi.Service, error) {
	if partitionable, ok := a.Service.(interface {
		Partition(string) (document.Service, error)
	}); ok {
		svc, err := partitionable.Partition(name)
		if err != nil {
			return nil, err
		}
		return AdaptTo(svc), nil
	}
	return nil, errors.New("document service does not support partitioning")
}

// Drop satisfies storageapi.DroppableService.
func (a adapter) Drop(ctx context.Context) error {
	droppable, ok := a.Service.(document.DroppableService)
	if !ok {
		return errors.New("document service does not support dropping")
	}
	err := droppable.Drop(ctx)
	switch {
	case errors.Is(err, document.ErrPermissionDenied):
		return storageapi.ErrPermissionDenied
	}
	return err
}

// ApplyBatch satisfies storageapi.BatchWriter.
func (a adapter) ApplyBatch(
	ctx context.Context, ops []storageapi.BatchOp,
) ([]storageapi.BatchOpResult, error) {
	writer, ok := a.Service.(document.BatchWriter)
	if !ok {
		return nil, errors.New("document service does not support batching")
	}
	svcOps := make([]document.BatchOp, len(ops))
	for i, op := range ops {
		opType, err := batchOpType(op.Type)
		if err != nil {
			return nil, err
		}
		svcOps[i] = document.BatchOp{
			Type: opType,
			ID:   op.ID,
			Doc:  op.Doc,
		}
		for _, update := range op.Updates {
			svcOps[i].Updates = append(svcOps[i].Updates, document.Update{
				FieldPath: update.FieldPath,
				Value:     update.Value,
			})
		}
		for _, precond := range op.Preconditions {
			svcOps[i].Preconditions = append(
				svcOps[i].Preconditions, document.Precondition{
					FieldPath: precond.FieldPath,
					Value:     precond.Value,
				})
		}
	}
	svcResults, err := writer.ApplyBatch(ctx, svcOps)
	if err != nil {
		if errors.Is(err, document.ErrPermissionDenied) {
			return nil, storageapi.ErrPermissionDenied
		}
		return nil, err
	}
	results := make([]storageapi.BatchOpResult, len(svcResults))
	for i, result := range svcResults {
		switch {
		case errors.Is(result.Err, document.ErrAlreadyExists):
			results[i].Err = storageapi.ErrAlreadyExists
		case errors.Is(result.Err, document.ErrNotFound):
			results[i].Err = storageapi.ErrNotFound
		case errors.Is(result.Err, document.ErrPreconditionFailed):
			results[i].Err = storageapi.ErrPreconditionFailed
		default:
			results[i].Err = result.Err
		}
	}
	return results, nil
}

func batchOpType(t storageapi.BatchOpType) (document.BatchOpType, error) {
	switch t {
	case storageapi.BatchCreate:
		return document.BatchCreate, nil
	case storageapi.BatchSet:
		return document.BatchSet, nil
	case storageapi.BatchUpdate:
		return document.BatchUpdate, nil
	case storageapi.BatchDelete:
		return document.BatchDelete, nil
	default:
		return 0, fmt.Errorf("unknown batch operation %d", t)
	}
}

type wrap struct {
	storageapi.Service
}

func (a wrap) List(ctx context.Context, filters []document.Filter) (
	document.Iterator, error,
) {
	var svcFilters []storageapi.Filter
	for _, filter := range filters {
		svcFilters = append(svcFilters, storageapi.Filter{
			Field: storageapi.Field{
				FieldPath: filter.FieldPath,
				Value:     filter.Value,
			},
			Op: storageapi.Op(filter.Op),
		})
	}
	it, err := a.Service.List(ctx, svcFilters)
	if err == nil {
		return it, err
	}
	switch {
	case errors.Is(err, storageapi.ErrPermissionDenied):
		return nil, document.ErrPermissionDenied
	}
	return it, err
}

func (a wrap) Create(ctx context.Context, ID string, doc any) error {
	err := a.Service.Create(ctx, ID, doc)
	switch {
	case errors.Is(err, storageapi.ErrAlreadyExists):
		return document.ErrAlreadyExists
	case errors.Is(err, storageapi.ErrPermissionDenied):
		return document.ErrPermissionDenied
	}
	return err
}

func (a wrap) Update(
	ctx context.Context, ID string,
	updates []document.Update, precond ...document.Precondition,
) error {
	var svcUpdates []storageapi.Update
	for _, filter := range updates {
		svcUpdates = append(svcUpdates, storageapi.Update{
			FieldPath: filter.FieldPath,
			Value:     filter.Value,
		})
	}
	var svcPreconds []storageapi.Precondition
	for _, filter := range precond {
		svcPreconds = append(svcPreconds, storageapi.Precondition{
			FieldPath: filter.FieldPath,
			Value:     filter.Value,
		})
	}
	err := a.Service.Update(ctx, ID, svcUpdates, svcPreconds...)
	switch {
	case errors.Is(err, storageapi.ErrPermissionDenied):
		return document.ErrPermissionDenied
	case errors.Is(err, storageapi.ErrNotFound):
		return document.ErrNotFound
	case errors.Is(err, storageapi.ErrPreconditionFailed):
		return document.ErrPreconditionFailed
	}
	return err
}

func (a wrap) Get(ctx context.Context, ID string, doc any) error {
	err := a.Service.Get(ctx, ID, doc)
	switch {
	case errors.Is(err, storageapi.ErrPermissionDenied):
		return document.ErrPermissionDenied
	case errors.Is(err, storageapi.ErrNotFound):
		return document.ErrNotFound
	}
	return err
}

func (a wrap) Delete(ctx context.Context, ID string) error {
	err := a.Service.Delete(ctx, ID)
	switch {
	case errors.Is(err, storageapi.ErrPermissionDenied):
		return document.ErrPermissionDenied
	}
	return err
}

// Drop satisfies document.DroppableService.
func (a wrap) Drop(ctx context.Context) error {
	droppable, ok := a.Service.(storageapi.DroppableService)
	if !ok {
		return errors.New("storage service does not support dropping")
	}
	err := droppable.Drop(ctx)
	switch {
	case errors.Is(err, storageapi.ErrPermissionDenied):
		return document.ErrPermissionDenied
	}
	return err
}
