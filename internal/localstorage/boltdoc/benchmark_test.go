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

package boltdoc_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/doctoml"

	"unstable.build/rune/internal/localstorage/boltdoc"
)

type benchmarkDoc struct {
	Version int64
	Title   string
	Active  bool
	Tags    []string
	Chunks  []benchmarkChunk
}

type benchmarkChunk struct {
	Index int64
	Name  string
	Body  string
	Score int64
}

func BenchmarkBoltMarshaler(b *testing.B) {
	docs := []struct {
		name string
		doc  benchmarkDoc
	}{
		{name: "Small", doc: newBenchmarkDoc(2, 64)},
		{name: "Large", doc: newBenchmarkDoc(256, 512)},
	}
	marshalers := []struct {
		name      string
		marshaler docmarshal.Marshaler
	}{
		{name: "TOML", marshaler: doctoml.Marshaler()},
		{name: "BSON", marshaler: docbson.Marshaler()},
	}

	for _, docCase := range docs {
		for _, marshalerCase := range marshalers {
			b.Run(docCase.name+"/"+marshalerCase.name+"/Set", func(b *testing.B) {
				benchmarkBoltSet(b, marshalerCase.marshaler, docCase.doc)
			})
			b.Run(docCase.name+"/"+marshalerCase.name+"/Get", func(b *testing.B) {
				benchmarkBoltGet(b, marshalerCase.marshaler, docCase.doc)
			})
			b.Run(docCase.name+"/"+marshalerCase.name+"/UpdateVersion", func(b *testing.B) {
				benchmarkBoltUpdateVersion(b, marshalerCase.marshaler, docCase.doc)
			})
		}
	}
}

func benchmarkBoltSet(b *testing.B, marshaler docmarshal.Marshaler, doc benchmarkDoc) {
	ctx := context.Background()
	svc := openBenchmarkService(b, marshaler)
	encodedSize := measureEncodedSize(b, marshaler, doc)

	b.ReportAllocs()
	b.ResetTimer()
	b.ReportMetric(float64(encodedSize), "encoded_B/doc")
	for i := 0; i < b.N; i++ {
		doc.Version = int64(i)
		if err := svc.Set(ctx, "doc", &doc); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkBoltGet(b *testing.B, marshaler docmarshal.Marshaler, doc benchmarkDoc) {
	ctx := context.Background()
	svc := openBenchmarkService(b, marshaler)
	if err := svc.Set(ctx, "doc", &doc); err != nil {
		b.Fatal(err)
	}
	encodedSize := measureEncodedSize(b, marshaler, doc)

	b.ReportAllocs()
	b.ResetTimer()
	b.ReportMetric(float64(encodedSize), "encoded_B/doc")
	for i := 0; i < b.N; i++ {
		var got benchmarkDoc
		if err := svc.Get(ctx, "doc", &got); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkBoltUpdateVersion(b *testing.B, marshaler docmarshal.Marshaler, doc benchmarkDoc) {
	ctx := context.Background()
	svc := openBenchmarkService(b, marshaler)
	doc.Version = 0
	if err := svc.Set(ctx, "doc", &doc); err != nil {
		b.Fatal(err)
	}
	encodedSize := measureEncodedSize(b, marshaler, doc)

	versionPath := []string{"Version"}
	updates := []storageapi.Update{{FieldPath: versionPath}}
	preconds := []storageapi.Precondition{{FieldPath: versionPath}}

	b.ReportAllocs()
	b.ResetTimer()
	b.ReportMetric(float64(encodedSize), "encoded_B/doc")
	for i := 0; i < b.N; i++ {
		updates[0].Value = int64(i + 1)
		preconds[0].Value = int64(i)
		if err := svc.Update(ctx, "doc", updates, preconds...); err != nil {
			b.Fatal(err)
		}
	}
}

func openBenchmarkService(b *testing.B, marshaler docmarshal.Marshaler) storageapi.Service {
	b.Helper()
	svc, err := boltdoc.New(filepath.Join(b.TempDir(), "rune.db"), marshaler)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = svc.Close() })
	return svc
}

func measureEncodedSize(b *testing.B, marshaler docmarshal.Marshaler, doc benchmarkDoc) int {
	b.Helper()
	encoded, err := marshaler.Marshal(&doc)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(encoded)))
	return len(encoded)
}

func newBenchmarkDoc(chunks, bodyBytes int) benchmarkDoc {
	body := strings.Repeat("x", bodyBytes)
	doc := benchmarkDoc{
		Version: 1,
		Title:   fmt.Sprintf("benchmark-%d-%d", chunks, bodyBytes),
		Active:  true,
		Tags:    []string{"rune", "storage", "benchmark"},
		Chunks:  make([]benchmarkChunk, chunks),
	}
	for i := range doc.Chunks {
		doc.Chunks[i] = benchmarkChunk{
			Index: int64(i),
			Name:  fmt.Sprintf("chunk-%03d", i),
			Body:  body,
			Score: int64(i * 3),
		}
	}
	return doc
}
