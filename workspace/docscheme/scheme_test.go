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

package docscheme

import (
	"context"
	"errors"
	"fmt"
	"io"

	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal/docjson"
	"github.com/unstablebuild/blue/document/docmarshal/docyaml"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

type testStruct struct {
	Id        string
	Content   string
	UpdatedAt time.Time
	UpdatedBy string
}

func (t testStruct) ID() string {
	return t.Id
}

func (t testStruct) WithID(id string) testStruct {
	t.Id = id
	return t
}

func (t testStruct) UpdatedTime() time.Time {
	return t.UpdatedAt
}

func (t testStruct) WithUpdatedTime(now time.Time) testStruct {
	t.UpdatedAt = now
	return t
}

func (t testStruct) WithUpdatedBy(author string) testStruct {
	t.UpdatedBy = author
	return t
}

func TestDocumentOpen(t *testing.T) {
	t.Run("file should have a default template after Open", func(t *testing.T) {
		ctx := context.Background()

		workspaceURI, err := workspaceapi.ParseURI("inmemory:///tmp")
		require.NoError(t, err)
		marshaler := docyaml.Marshaler()
		svc := document.NewInMemoryServiceWithMarshaler(marshaler)
		scheme, err := Scheme[testStruct](workspaceURI, svc,
			marshaler, errors.New("missing id"), "author")(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		f, werr := scheme.Open("a", os.O_CREATE|os.O_RDWR, 0)
		require.Nil(t, werr)

		data, err := io.ReadAll(f)
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(string(data), "id: a\ncontent: \"\""))
	})

	t.Run("swap at rest should be populated", func(t *testing.T) {
		ctx := context.Background()
		workspaceURI, err := workspaceapi.ParseURI("inmemory:///tmp")
		require.NoError(t, err)
		marshaler := docjson.Marshaler()

		svc := document.NewInMemoryServiceWithMarshaler(marshaler)

		// two schemes so there's no usage of the cached file in memory
		scheme1, err := Scheme[testStruct](workspaceURI, svc,
			marshaler, errors.New("missing id"), "author")(ctx, config.NopConfig(),
			workspaceURI)
		scheme2, err := Scheme[testStruct](workspaceURI, svc,
			marshaler, errors.New("missing id"), "author")(ctx, config.NopConfig(),
			workspaceURI)
		defer scheme1.Close()
		defer scheme2.Close()

		uri, err := scheme1.URI(".")
		require.NoError(t, err)
		wp1 := workspace.NewSchemeWorkspace(uri, scheme1)
		wp2 := workspace.NewSchemeWorkspace(uri, scheme2)
		fileuri := workspaceapi.Join(uri, "dataAtRestTest")
		swapfileuri := workspaceapi.Join(uri, ".dataAtRestTest.swp")
		swapDir := workspaceapi.Join(uri, ".")

		buf := cell.NewBuffer()
		_, err = wp1.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)
		_, err = write(buf, []byte("short"))

		// write is async so wait for it
		time.Sleep(2000 * time.Millisecond)
		require.NoError(t, err)

		buf = cell.NewBuffer()
		fc2, err := wp2.Recover(fileuri, swapfileuri, buf, true)
		require.NoError(t, err)
		require.NoError(t, fc2.Flush())

		f, werr := scheme2.Open("dataAtRestTest", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "short", string(data))

		require.NoError(t, fc2.Close())
		require.NoError(t, f.Close())
	})
}

func readAll(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var temp testStruct
	err = docjson.Marshaler().Unmarshal(data, &temp)
	if err != nil {
		return nil, err
	}

	return []byte(temp.Content), nil
}

func write(buf *cell.Buffer, data []byte) (int, error) {
	var doc testStruct

	err := docjson.Marshaler().Unmarshal([]byte(buf.String()), &doc)
	if err != nil {
		return 0, fmt.Errorf("unmarshal: %v: %q", err, buf.String())
	}
	var builder strings.Builder
	builder.WriteString(doc.Content)
	builder.Write(data)
	doc.Content = builder.String()

	data, err = docjson.Marshaler().Marshal(doc)
	if err != nil {
		return 0, fmt.Errorf("marshal: %v", err)
	}

	end := term.Coordinates{Y: buf.Rows(), X: buf.Columns(buf.Rows()-1) + 1}
	buf.Edit(context.Background(), term.Coordinates{}, end, string(data))
	return len(data), nil
}
