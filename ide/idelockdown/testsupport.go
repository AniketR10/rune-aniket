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

package idelockdown

import (
	"context"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

// SeedQualifyingUsage writes a usage history that satisfies the
// lockdown policy under the currently-compiled enforcement knobs: one
// sample per usage bucket across enough windows to exceed lockdownRun,
// so tests lock regardless of whether the knobs are production-scale
// or compressed for manual testing. Intended for tests only.
func SeedQualifyingUsage(ctx context.Context, storage storageapi.Service, now time.Time) error {
	span := lockdownRun + qualifyWindow
	end := bucketStart(now, usageBucket)
	var samples []string
	for t := end.Add(-span); !t.After(end); t = t.Add(usageBucket) {
		samples = append(samples, t.UTC().Format(sampleLayout))
	}
	doc := usageDoc{Kind: usageDocKind, Version: 1, Usage: samples}
	store := storageapi.WithPartition(storage, Partition)
	return store.Create(ctx, usageDocID, &doc)
}
