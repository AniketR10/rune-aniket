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

package headless

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectRequiresPluginEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name    string
		socket  string
		datadir string
	}{
		{name: "no socket", datadir: "/tmp/data"},
		{name: "no datadir", socket: "unix:///tmp/rune.sock"},
		{name: "neither"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RUNE_SOCKET", tc.socket)
			t.Setenv("RUNE_DATADIR", tc.datadir)

			w, err := Connect()

			assert.Nil(t, w)
			assert.ErrorIs(t, err, ErrNotInRune)
			assert.Contains(t, err.Error(),
				"must be invoked from a Rune plugin command (! or !!)")
		})
	}
}

func TestConnectDecodesCertificate(t *testing.T) {
	t.Setenv("RUNE_SOCKET", "unix:///tmp/rune.sock")
	t.Setenv("RUNE_DATADIR", t.TempDir())
	t.Setenv("RUNE_TOKEN", "sekret")
	t.Setenv("RUNE_CERT", base64.StdEncoding.EncodeToString(selfSignedPEM(t)))

	w, err := Connect()

	require.NoError(t, err)
	require.NotNil(t, w)
}

func TestConnectRejectsMalformedCertificate(t *testing.T) {
	t.Setenv("RUNE_SOCKET", "unix:///tmp/rune.sock")
	t.Setenv("RUNE_DATADIR", t.TempDir())
	t.Setenv("RUNE_CERT", "not base64!!")

	w, err := Connect()

	assert.Nil(t, w)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode RUNE_CERT")
}

func selfSignedPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "rune-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
