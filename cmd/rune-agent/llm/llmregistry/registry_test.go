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

package llmregistry_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func collectModels(t *testing.T, it iterator.Iterator[llmregistry.ModelEntry]) []llmregistry.ModelEntry {
	t.Helper()
	entries, err := iterator.ToSlice(context.Background(), it)
	if err != nil {
		t.Fatalf("collecting models: %v", err)
	}
	return entries
}

func TestStaticRegistry_empty(t *testing.T) {
	r := llmregistry.NewStatic()
	if models := collectModels(t, r.Models()); len(models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(models))
	}
	_, ok := r.Get(context.Background(), "anything")
	if ok {
		t.Fatal("expected Get to return false for empty registry")
	}
}

func TestStaticRegistry_register_and_get(t *testing.T) {
	r := llmregistry.NewStatic()
	r.Register(
		llmregistry.ModelEntry{Name: "m1", Provider: "openai", ContextWindow: 128000},
		llmregistry.ModelEntry{Name: "m2", Provider: "anthropic", ContextWindow: 200000, BaseURL: "https://api.anthropic.com/v1/"},
	)

	if models := collectModels(t, r.Models()); len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	e, ok := r.Get(context.Background(), "m1")
	if !ok {
		t.Fatal("expected Get m1 to succeed")
	}
	if e.Provider != "openai" {
		t.Fatalf("expected provider openai, got %s", e.Provider)
	}
	if e.ContextWindow != 128000 {
		t.Fatalf("expected context window 128000, got %d", e.ContextWindow)
	}

	e, ok = r.Get(context.Background(), "m2")
	if !ok {
		t.Fatal("expected Get m2 to succeed")
	}
	if e.BaseURL != "https://api.anthropic.com/v1/" {
		t.Fatalf("expected base URL, got %q", e.BaseURL)
	}
}

func TestStaticRegistry_sorted(t *testing.T) {
	r := llmregistry.NewStatic()
	r.Register(
		llmregistry.ModelEntry{Name: "zeta"},
		llmregistry.ModelEntry{Name: "alpha"},
		llmregistry.ModelEntry{Name: "mu"},
	)

	models := collectModels(t, r.Models())
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name
	}
	want := []string{"alpha", "mu", "zeta"}
	for i, n := range names {
		if n != want[i] {
			t.Fatalf("expected %v, got %v", want, names)
		}
	}
}

func TestStaticRegistry_overwrite(t *testing.T) {
	r := llmregistry.NewStatic()
	r.Register(llmregistry.ModelEntry{Name: "m1", Provider: "openai", ContextWindow: 100})
	r.Register(llmregistry.ModelEntry{Name: "m1", Provider: "anthropic", ContextWindow: 200})

	e, ok := r.Get(context.Background(), "m1")
	if !ok {
		t.Fatal("expected Get to succeed")
	}
	if e.Provider != "anthropic" || e.ContextWindow != 200 {
		t.Fatalf("expected overwritten entry, got %+v", e)
	}
}

func TestStaticRegistry_concurrent_access(t *testing.T) {
	r := llmregistry.NewStatic()

	var wg sync.WaitGroup
	// Concurrent writers
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 100 {
				name := fmt.Sprintf("model-%d-%d", i, j)
				r.Register(llmregistry.ModelEntry{
					Name:          name,
					Provider:      "openai",
					ContextWindow: 128000,
				})
			}
		}()
	}
	// Concurrent readers
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				r.Models()
				r.Get(context.Background(), "model-0-0")
			}
		}()
	}
	wg.Wait()

	// Verify all models were registered.
	models := collectModels(t, r.Models())
	if len(models) != 1000 {
		t.Fatalf("expected 1000 models, got %d", len(models))
	}
}

func TestCompositeRegistry_empty(t *testing.T) {
	c := llmregistry.NewComposite()
	if models := collectModels(t, c.Models()); len(models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(models))
	}
	_, ok := c.Get(context.Background(), "anything")
	if ok {
		t.Fatal("expected Get to return false for empty composite")
	}
}

func TestCompositeRegistry_merges(t *testing.T) {
	r1 := llmregistry.NewStatic()
	r1.Register(
		llmregistry.ModelEntry{Name: "m1", Provider: "openai", ContextWindow: 100},
		llmregistry.ModelEntry{Name: "m2", Provider: "openai", ContextWindow: 200},
	)

	r2 := llmregistry.NewStatic()
	r2.Register(
		llmregistry.ModelEntry{Name: "m3", Provider: "anthropic", ContextWindow: 300},
	)

	c := llmregistry.NewComposite(r1, r2)
	models := collectModels(t, c.Models())
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}

	e, ok := c.Get(context.Background(), "m3")
	if !ok {
		t.Fatal("expected Get m3 to succeed")
	}
	if e.Provider != "anthropic" {
		t.Fatalf("expected anthropic, got %s", e.Provider)
	}
}

func TestCompositeRegistry_preserves_sub_registry_order(t *testing.T) {
	r1 := llmregistry.NewStatic()
	r1.Register(
		llmregistry.ModelEntry{Name: "beta", Provider: "openai"},
		llmregistry.ModelEntry{Name: "alpha", Provider: "openai"},
	)
	r2 := llmregistry.NewStatic()
	r2.Register(
		llmregistry.ModelEntry{Name: "delta", Provider: "anthropic"},
		llmregistry.ModelEntry{Name: "gamma", Provider: "anthropic"},
	)

	c := llmregistry.NewComposite(r1, r2)
	models := collectModels(t, c.Models())
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = m.Name
	}
	// Each sub-registry is sorted; composite concatenates them in order.
	want := []string{"alpha", "beta", "delta", "gamma"}
	for i, n := range names {
		if n != want[i] {
			t.Fatalf("expected %v, got %v", want, names)
		}
	}
}

func TestCompositeRegistry_earlier_overrides(t *testing.T) {
	r1 := llmregistry.NewStatic()
	r1.Register(llmregistry.ModelEntry{Name: "m1", Provider: "openai", ContextWindow: 100})

	r2 := llmregistry.NewStatic()
	r2.Register(llmregistry.ModelEntry{Name: "m1", Provider: "ollama", ContextWindow: 999, BaseURL: "http://localhost:11434/v1/"})

	c := llmregistry.NewComposite(r1, r2)

	e, ok := c.Get(context.Background(), "m1")
	if !ok {
		t.Fatal("expected Get m1 to succeed")
	}
	if e.Provider != "openai" {
		t.Fatalf("expected openai (earlier registry wins), got %s", e.Provider)
	}
	if e.ContextWindow != 100 {
		t.Fatalf("expected context window 100, got %d", e.ContextWindow)
	}

	// Models() does not deduplicate; both entries are visible.
	models := collectModels(t, c.Models())
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
}
