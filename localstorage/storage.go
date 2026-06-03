// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package localstorage

import (
	"context"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/localstorage/boltdoc"
	"unstable.build/go-tui/localstorage/firstmover"
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
