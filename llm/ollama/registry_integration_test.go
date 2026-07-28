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

package ollama_test

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"unstable.build/go-tui/llm/ollama"
)

// TestIntegrationRegistry starts a real Ollama server (via
// `ollama serve`) and verifies that the registry can discover
// models from it.
func TestIntegrationRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ollamaBin, err := exec.LookPath("ollama")
	if err != nil {
		t.Skip("ollama binary not found; skipping integration test")
	}

	// Use a non-default port so we don't conflict with a user's
	// running instance.
	const port = "19434"
	baseURL := "http://127.0.0.1:" + port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, ollamaBin, "serve")
	cmd.Env = append(cmd.Environ(), "OLLAMA_HOST=127.0.0.1:"+port)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start ollama serve: %v", err)
	}
	defer func() {
		cancel()
		cmd.Wait() //nolint:errcheck
	}()

	// Wait for the server to become ready via a direct HTTP probe.
	reg := ollama.NewRegistry(baseURL)
	var ready bool
	client := &http.Client{Timeout: time.Second}
	for range 30 {
		resp, err := client.Get(baseURL + "/api/tags")
		if err == nil {
			_ = resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		t.Fatal("ollama server did not become ready in time")
	}

	// Verify Models returns entries.
	models := collectModels(t, reg.Models())
	t.Logf("ollama returned %d models", len(models))
	for _, m := range models {
		if m.Name == "" {
			t.Error("model entry has empty name")
		}
		if m.Provider != ollama.LLMProvider {
			t.Errorf("expected provider %q, got %q", ollama.LLMProvider, m.Provider)
		}
		if m.BaseURL != baseURL+"/v1/" {
			t.Errorf("unexpected base URL %q", m.BaseURL)
		}
		if m.ContextWindow == 0 {
			t.Errorf("model %q has zero context window", m.Name)
		}
	}

	// If models are available, verify Get works.
	if len(models) > 0 {
		name := models[0].Name
		e, ok := reg.Get(ctx, name)
		if !ok {
			t.Fatalf("Get(%q) returned false for a model from Models()", name)
		}
		if e.Name != name {
			t.Fatalf("Get returned wrong name: %q vs %q", e.Name, name)
		}
	}

	// Verify Get returns false for a model that doesn't exist.
	_, ok := reg.Get(ctx, "this-model-does-not-exist-"+fmt.Sprint(time.Now().UnixNano()))
	if ok {
		t.Fatal("Get returned true for a non-existent model")
	}

	// Sanity: registry continues to function.
	_ = reg
}
