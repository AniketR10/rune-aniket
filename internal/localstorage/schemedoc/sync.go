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

package schemedoc

import (
	"context"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

// Sync returns a mutual exclusion storageapi.Service.
// All calls are serialized preventing concurrent access to the given
// underlying Service.
func Sync(svc storageapi.Service) storageapi.Service {
	return SyncWithLocker(svc, new(sync.Mutex))
}

// SyncWithLocker returns a mutual exclusion storageapi.Service using the
// provided locker for synchronization.
func SyncWithLocker(svc storageapi.Service, locker sync.Locker) storageapi.Service {
	return &syncService{svc: svc, mu: locker}
}

type syncService struct {
	mu  sync.Locker
	svc storageapi.Service
}

func (s *syncService) Create(ctx context.Context, ID string, doc any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Create(ctx, ID, doc)
}

func (s *syncService) Set(ctx context.Context, ID string, doc any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Set(ctx, ID, doc)
}

func (s *syncService) Update(
	ctx context.Context, ID string, updates []storageapi.Update,
	preconds ...storageapi.Precondition,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Update(ctx, ID, updates, preconds...)
}

func (s *syncService) Get(ctx context.Context, ID string, doc any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Get(ctx, ID, doc)
}

func (s *syncService) Delete(ctx context.Context, ID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Delete(ctx, ID)
}

func (s *syncService) List(ctx context.Context, filters []storageapi.Filter) (storageapi.Iterator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.List(ctx, filters)
}

func (s *syncService) Partition(name string) (storageapi.Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	partitioned, err := s.svc.Partition(name)
	if err != nil {
		return nil, err
	}
	return SyncWithLocker(partitioned, s.mu), nil
}

func (s *syncService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc.Close()
}
