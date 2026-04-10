// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2026 Unstable Build, All Rights Reserved.
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
