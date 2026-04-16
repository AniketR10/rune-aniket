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

package main

import (
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"unstable.build/go-tui/cmd/rune-agent/extension"
	"unstable.build/go-tui/cmd/rune-agent/llm/anthropic"
	"unstable.build/go-tui/cmd/rune-agent/llm/gemini"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
	"unstable.build/go-tui/cmd/rune-agent/llm/ollama"
	"unstable.build/go-tui/cmd/rune-agent/llm/openai"
	"unstable.build/go-tui/debug"
)

var (
	// Tag is a compile-time variable
	Tag = "development"
	// Commit is a compile-time variable
	Commit = "HEAD"
	// Version is injected at compile time.
	Version string
)

func init() {
	Version = fmt.Sprintf("%s (HEAD is %s)", Tag, Commit)
}

func main() {
	go debug.CapturePanicReport(func() {

		err := http.ListenAndServe("localhost:6669", nil)
		if err != nil {
			slog.Warn("pprof server listen and serve", "error", err)
		}

	})

	// Build the composite model registry from all providers.
	static := llmregistry.NewStatic()
	openai.RegisterModels(static)
	anthropic.RegisterModels(static)
	gemini.RegisterModels(static)

	ollamaRegistry := ollama.NewRegistry("")
	registry := llmregistry.NewComposite(static, ollamaRegistry)

	defaultModel := openai.GPT5Dot4

	ext, metadata := extension.NewExtension(registry, defaultModel)
	err := extensionapi.ServeWorkspaceExtension(ext, metadata)
	if err != nil {
		slog.Error("serve extension", "error", err)
		os.Exit(1)
	}
}
