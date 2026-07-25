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

package idepkgtest

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/ProtonMail/go-crypto/openpgp"
	"unstable.build/go-tui/ide/pkgtrust"
)

// testEntity is the lazily generated PGP entity that signs every tarball the
// mock ReleaseManager returns. Generating it once keeps signing cheap across a
// package's tests while still exercising the real verification path.
var (
	testEntityOnce sync.Once
	testEntity     *openpgp.Entity
)

func sharedTestEntity() *openpgp.Entity {
	testEntityOnce.Do(func() {
		entity, err := openpgp.NewEntity("Rune Test Publisher", "", "test@rune.build", nil)
		if err != nil {
			panic(fmt.Sprintf("idepkgtest: generate test entity: %v", err))
		}
		testEntity = entity
	})
	return testEntity
}

// TrustStore returns a pkgtrust.Store that trusts the entity the mock
// ReleaseManager signs bundles with, so idepkg.Manager tests exercise
// mandatory verification without a production backdoor.
func TrustStore() *pkgtrust.Store {
	return pkgtrust.NewStoreWithKeyring(openpgp.EntityList{sharedTestEntity()})
}

// signTestPayload signs payload with the shared test entity and returns the
// signing key ID and armored detached signature for bundle metadata.
func signTestPayload(payload []byte) (keyID, signature string, err error) {
	entity := sharedTestEntity()
	var sig bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&sig, entity, bytes.NewReader(payload), nil); err != nil {
		return "", "", fmt.Errorf("sign test payload: %w", err)
	}
	return fmt.Sprintf("%016X", entity.PrimaryKey.KeyId), sig.String(), nil
}

func cloneMetadata(m map[string]string) map[string]string {
	out := make(map[string]string, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	return out
}
