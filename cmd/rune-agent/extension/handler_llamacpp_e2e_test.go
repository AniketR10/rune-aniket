// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY.

package extension

// End-to-end coverage for the handler's local llama.cpp integration. The
// suite is gated on RUNE_LLAMACPP_E2E=1 (or RUNE_LLAMACPP_E2E_MODEL=<path>)
// and drives handleChat/newChatService through the real llm.Service
// returned by newLlamaCppService — no mock hooks. The cached model is
// shared with the llamacpp package's own e2e suite via ${UserCacheDir}/
// rune/llamacpp-e2e.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/llm/llamacpp"
	"unstable.build/go-tui/cmd/rune-agent/llm/llamacpp/ociregistry"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
)

const extE2EReference = "unsloth/gemma-3n-E2B-it-GGUF:Q2_K"

// resolveE2EModelPath returns the on-disk path of the shared e2e GGUF,
// pulling it via our ociregistry client on first use. The suite is
// skipped when the env flags say not to exercise llama.cpp.
func resolveE2EModelPath(t *testing.T) string {
	t.Helper()
	if os.Getenv("RUNE_LLAMACPP_E2E") != "1" && os.Getenv("RUNE_LLAMACPP_E2E_MODEL") == "" {
		t.Skip("llamacpp e2e: set RUNE_LLAMACPP_E2E=1 (or RUNE_LLAMACPP_E2E_MODEL=/path/to/model.gguf) to run")
	}
	if p := os.Getenv("RUNE_LLAMACPP_E2E_MODEL"); p != "" {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("RUNE_LLAMACPP_E2E_MODEL=%q: %v", p, err)
		}
		return p
	}
	baseDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("UserCacheDir: %v", err)
	}
	cacheDir := filepath.Join(baseDir, "rune", "llamacpp-e2e")
	cache, err := ociregistry.OpenCache(cacheDir)
	if err != nil {
		t.Fatalf("OpenCache(%s): %v", cacheDir, err)
	}
	ref, err := ociregistry.ParseReferenceWithDefault(extE2EReference, "huggingface.co")
	if err != nil {
		t.Fatalf("parse %q: %v", extE2EReference, err)
	}
	if pr, err := ociregistry.Resolve(cache, ref); err == nil && pr.ModelPath != "" {
		if _, err := os.Stat(pr.ModelPath); err == nil {
			return pr.ModelPath
		}
	}
	if os.Getenv("RUNE_LLAMACPP_E2E_OFFLINE") == "1" {
		t.Skipf("llamacpp e2e: offline and no cached model at %s", cacheDir)
	}
	t.Logf("llamacpp e2e: downloading %s into %s (first-run only)", ref.String(), cacheDir)
	client := ociregistry.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	pr, err := client.Pull(ctx, cache, ref, nil)
	if err != nil {
		t.Skipf("llamacpp e2e: pull %s: %v", ref.String(), err)
	}
	if pr.ModelPath == "" {
		t.Skipf("llamacpp e2e: no model layer in manifest for %s", ref.String())
	}
	return pr.ModelPath
}

// TestE2E_AIEditorHandler_newChatService_surfacesLoadingNotification
// exercises the notification + progress plumbing in newChatService by
// driving it against a real llama.cpp model. We don't assert on the
// exact progress values — llama.cpp decides how often to fire the
// callback — only that:
//   - newChatService returns a real, usable llm.Service (no mock hook)
//   - a "Loading local model..." notification is emitted
//   - at least one progress update is posted before the service is ready
func TestE2E_AIEditorHandler_newChatService_surfacesLoadingNotification(t *testing.T) {
	modelPath := resolveE2EModelPath(t)
	modelName := filepath.Base(modelPath)

	deps := newTestAIEditorHandler(t, &agentMockService{})
	noti := &capturingNotifications{}
	deps.handler.n = noti

	reg := llmregistry.NewStatic()
	reg.Register(llmregistry.ModelEntry{
		Name:          modelName,
		Provider:      llamacpp.LLMProvider,
		ContextWindow: 2048,
		BaseURL:       modelPath,
	})
	deps.handler.modelRegistry = reg
	deps.handler.defaultModel = modelName
	deps.handler.queryDefaultModel = modelName

	svc, err := deps.handler.newChatService(modelName)
	require.NoError(t, err)
	require.NotNil(t, svc)
	t.Cleanup(func() {
		if closer, ok := svc.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	})

	noti.mu.Lock()
	defer noti.mu.Unlock()
	require.NotEmpty(t, noti.notified, "expected at least one notification")
	foundLoading := false
	for _, m := range noti.notified {
		if strings.HasPrefix(m, "Loading local model") {
			foundLoading = true
			break
		}
	}
	assert.True(t, foundLoading,
		"no 'Loading local model...' notification; got %v", noti.notified)
	assert.NotEmpty(t, noti.progress, "expected at least one progress update")
}
