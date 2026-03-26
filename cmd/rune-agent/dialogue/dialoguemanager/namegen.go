// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package dialoguemanager

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

const maxNameRetries = 10

// GenerateUniqueID generates a petname-based dialogue ID with the
// given prefix, checking the store to ensure no existing dialogue
// has the same ID. The resulting format is "{prefix}{petname}".
// After maxNameRetries collisions it appends a short random suffix
// to guarantee uniqueness.
func GenerateUniqueID(ctx context.Context, store Store, prefix string) string {
	for range maxNameRetries {
		id := prefix + petname.Generate(2, "-")
		_, err := store.Get(ctx, id)
		switch {
		case errors.Is(err, storageapi.ErrNotFound):
			return id // unique
		case err != nil:
			slog.Warn("GenerateUniqueID: store check failed, using generated ID",
				"id", id, "error", err)
			return id
		}
		// err == nil means collision — retry.
	}
	// Fallback: append random suffix.
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return prefix + petname.Generate(2, "-") + "-" + fmt.Sprintf("%x", buf[:])
}
