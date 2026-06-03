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

	"unstable.build/go-tui/localstorage/boltdoc"
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
