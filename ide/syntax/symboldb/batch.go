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

package symboldb

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// symbolWrite is the write one name needs so that a file's stored
// contribution matches what the file now defines, plus the before-state
// the name index needs once the write lands.
type symbolWrite struct {
	name      string
	locs      []symbolLoc
	merged    []symbolLoc
	wasListed bool
	// wrote is true while op still has to be applied, and stays true
	// after it lands so the name index takes the cheap path.
	wrote bool
	op    storageapi.BatchOp
}

// upsertSymbolBatch writes every symbol record of a file in one round
// trip: it reads the affected documents, stages the writes they need,
// and applies them in a single batch, re-staging only the ones the
// store rejected because another writer moved first.
func (p *Parser) upsertSymbolBatch(
	ctx context.Context, us string, oldNames []string,
	symbols map[string][]symbolLoc, force bool,
) {
	if ctx.Err() != nil {
		return
	}
	writes := make([]symbolWrite, 0, len(oldNames)+len(symbols))
	for _, name := range oldNames {
		if _, ok := symbols[name]; ok {
			continue
		}
		if w, ok := p.stageSymbolWrite(ctx, name, us, nil); ok {
			writes = append(writes, w)
		}
	}
	// A map iteration would make the batch — and so the bolt page
	// writes and any conflict ordering — differ run to run.
	for _, name := range slices.Sorted(maps.Keys(symbols)) {
		if w, ok := p.stageSymbolWrite(ctx, name, us, symbols[name]); ok {
			writes = append(writes, w)
		}
	}
	if !p.applySymbolWrites(ctx, us, writes) {
		// The store turned out not to support batching after all;
		// replay this file through the per-operation path.
		p.upsertSymbols(ctx, us, oldNames, symbols, force)
		return
	}
	for _, w := range writes {
		p.syncNameIndex(ctx, w.name, w.merged, w.wrote && !force, w.wasListed)
	}
}

// stageSymbolWrite reads name's stored document and computes the write
// it needs, if any. It reports false when the name needs no attention
// at all, either because nothing is stored and nothing would be, or
// because the read failed.
func (p *Parser) stageSymbolWrite(
	ctx context.Context, name, us string, locs []symbolLoc,
) (symbolWrite, bool) {
	w := symbolWrite{name: name, locs: locs}
	var doc symbolDoc
	err := p.symbols.Get(ctx, name, &doc)
	switch {
	case errors.Is(err, storageapi.ErrNotFound):
		if len(locs) == 0 {
			// Nothing stored and nothing to store: a tombstone for a
			// symbol that was never stored would be noise.
			return symbolWrite{}, false
		}
		w.merged, w.wrote = locs, true
		w.op = storageapi.BatchOp{
			Type: storageapi.BatchCreate, ID: name,
			Doc: symbolDoc{Name: name, Locs: locs, Version: 1},
		}
	case err != nil:
		if ctx.Err() == nil {
			log.Errorf("symboldb: read symbol %q: %v", name, err)
		}
		return symbolWrite{}, false
	default:
		w.wasListed = listed(doc.Locs)
		w.merged = mergeLocs(doc.Locs, us, locs)
		if sameLocs(doc.Locs, w.merged) {
			return w, true
		}
		// A doc written before the counter existed stores no Version
		// field; nil matches its absence where 0 would not.
		var current any
		if doc.Version != 0 {
			current = doc.Version
		}
		w.wrote = true
		w.op = storageapi.BatchOp{
			Type: storageapi.BatchUpdate, ID: name,
			Updates: []storageapi.Update{
				{FieldPath: []string{"Locs"}, Value: w.merged},
				{FieldPath: []string{"Version"}, Value: doc.Version + 1},
			},
			Preconditions: []storageapi.Precondition{
				{FieldPath: []string{"Version"}, Value: current},
			},
		}
	}
	return w, true
}

// applySymbolWrites applies the staged writes, retrying the operations
// a concurrent writer invalidated. It reports false when the store does
// not implement batching, leaving the writes unapplied.
func (p *Parser) applySymbolWrites(
	ctx context.Context, us string, writes []symbolWrite,
) bool {
	pending := make([]int, 0, len(writes))
	for i := range writes {
		if writes[i].wrote {
			pending = append(pending, i)
		}
	}
	for attempt := uint(1); len(pending) > 0; attempt++ {
		ops := make([]storageapi.BatchOp, len(pending))
		for i, idx := range pending {
			ops[i] = writes[idx].op
		}
		results, err := p.batch.ApplyBatch(ctx, ops)
		if err != nil {
			if status.Code(err) == codes.Unimplemented {
				p.batch = nil
				return false
			}
			if ctx.Err() == nil {
				log.Errorf("symboldb: persist %d symbols of %q: %v",
					len(ops), us, err)
			}
			for _, idx := range pending {
				writes[idx].wrote = false
			}
			return true
		}

		var stale []int
		for i, result := range results {
			idx := pending[i]
			switch {
			case result.Err == nil:
			case errors.Is(result.Err, storageapi.ErrPreconditionFailed),
				errors.Is(result.Err, storageapi.ErrAlreadyExists),
				errors.Is(result.Err, storageapi.ErrNotFound):
				stale = append(stale, idx)
			default:
				if ctx.Err() == nil {
					log.Errorf("symboldb: persist symbol %q: %v",
						writes[idx].name, result.Err)
				}
				writes[idx].wrote = false
			}
		}
		if len(stale) == 0 {
			return true
		}
		sleep, stop := retryStrategy(attempt)
		if stop {
			for _, idx := range stale {
				log.Errorf("symboldb: persist symbol %q: %v",
					writes[idx].name, storageapi.ErrPreconditionFailed)
				writes[idx].wrote = false
			}
			return true
		}
		select {
		case <-ctx.Done():
			return true
		case <-time.After(sleep):
		}

		pending = pending[:0]
		for _, idx := range stale {
			w, ok := p.stageSymbolWrite(
				ctx, writes[idx].name, us, writes[idx].locs)
			if !ok {
				writes[idx].wrote = false
				continue
			}
			writes[idx] = w
			if w.wrote {
				pending = append(pending, idx)
			}
		}
	}
	return true
}
