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

package font

import (
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"unstable.build/go-tui/term/gui/font/builtinfont"
)

// countingReaderAt records how many bytes were read through it.
type countingReaderAt struct {
	r     io.ReaderAt
	bytes atomic.Int64
}

func (c *countingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.r.ReadAt(p, off)
	c.bytes.Add(int64(n))
	return n, err
}

// TestReadMetadataDoesNotReadWholeFile guards against font discovery
// regressing to slurping every candidate file: system collections are
// hundreds of megabytes and only their family names are needed.
func TestReadMetadataDoesNotReadWholeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "NotoColorEmoji.ttf")
	require.NoError(t, os.WriteFile(path, builtinfont.EmojiTTF, 0o600))

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	info, err := f.Stat()
	require.NoError(t, err)

	counter := &countingReaderAt{r: f}
	meta, err := readMetadata(path, counter)
	require.NoError(t, err)
	require.NotEmpty(t, meta)
	assert.Equal(t, "Noto Color Emoji", meta[0].family)
	assert.Equal(t, path, meta[0].path)

	read := counter.bytes.Load()
	assert.Less(t, read, info.Size()/10,
		"read %d of %d bytes; metadata must not slurp the whole file",
		read, info.Size())
}
