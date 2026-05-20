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

// claudeimport compiles Claude Code conversations into a Go memory program.
//
// It reads JSONL conversation files from a Claude home directory, extracts
// durable memories via an LLM agent, and writes them as Go source files.
//
// Must be invoked from within a Rune workspace (RUNE_SOCKET set).
//
// Usage:
//
//	claudeimport [flags] [source]
//
// Source is the Claude home directory (default: ~/.claude).
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"golang.org/x/oauth2"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmarg"
	"unstable.build/go-tui/cmd/rune-agent/memory/claudememory"
	"unstable.build/go-tui/cmd/rune-agent/memory/dream"
	"unstable.build/go-tui/debug"
)

var errNotInRune = errors.New(
	"claudeimport must be invoked from a Rune workspace (RUNE_SOCKET not set)",
)

func main() {
	debug.StartPProfOnSignal()
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

type app struct {
	workspace *extensionapi.Workspace
}

func (a *app) getWorkspace() (*extensionapi.Workspace, error) {
	if a.workspace != nil {
		return a.workspace, nil
	}
	socket := os.Getenv("RUNE_SOCKET")
	if socket == "" {
		return nil, errNotInRune
	}
	datadir := os.Getenv("RUNE_DATADIR")
	if datadir == "" {
		return nil, errNotInRune
	}
	cfg := extensionapi.Config{
		Socket:  socket,
		DataDir: datadir,
	}
	if tok := os.Getenv("RUNE_TOKEN"); tok != "" {
		cfg.Token = &oauth2.Token{AccessToken: tok}
	}
	if cert := os.Getenv("RUNE_CERT"); cert != "" {
		data, err := base64.StdEncoding.DecodeString(cert)
		if err != nil {
			return nil, fmt.Errorf("decode cert: %w", err)
		}
		cfg.Certificate = data
	}
	meta := extensionapi.Metadata{
		ExtensionID: "claudeimport",
	}
	w, err := extensionapi.NewWorkspace(cfg, meta)
	if err != nil {
		return nil, err
	}
	a.workspace = w
	return w, nil
}

func newRootCmd() *cobra.Command {
	a := &app{}
	var (
		output      string
		model       string
		apiKey      string
		baseURL     string
		verbose     bool
		quiet       bool
		noAnimation bool
		preprocess  bool
	)

	cmd := &cobra.Command{
		Use:   "claudeimport [flags] [source]",
		Short: "Compile Claude Code conversations into a Go memory program",
		Long: `claudeimport reads Claude Code conversation files (.jsonl) and compiles
them into durable memories stored as Go source files.

The source argument is the Claude home directory (default: ~/.claude).
The output is a Go module containing categorized memory files.

Requires a running Rune workspace (RUNE_SOCKET environment variable).

Examples:
  claudeimport -o ./memories
  claudeimport -o ./memories ~/.claude
  claudeimport -E                         # list unprocessed conversations
  claudeimport -m claude-sonnet-4-6 -o ./memories`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			})))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			source := defaultClaudeHome()
			if len(args) > 0 {
				source = args[0]
			}

			if preprocess {
				return runPreprocess(cmd.Context(), source)
			}

			return a.runCompile(cmd.Context(), source, output, model, apiKey, baseURL, quiet, noAnimation)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "./memories", "Output directory for compiled memories")
	cmd.Flags().StringVarP(&model, "model", "m", "claude-opus-4-6", "LLM model to use")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key (default: $OPENAI_API_KEY or $ANTHROPIC_API_KEY)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Custom API base URL")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress all progress output")
	cmd.Flags().BoolVar(&noAnimation, "no-animation", false, "Plain line output instead of animated progress")
	cmd.Flags().BoolVarP(&preprocess, "preprocess", "E", false, "Preprocess only: list unprocessed conversations and exit")

	return cmd
}

// runPreprocess lists conversations that would be compiled, without
// invoking the LLM or connecting to the workspace. Analogous to cc -E.
func runPreprocess(ctx context.Context, source string) error {
	store := claudememory.NewStore(source)
	it, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("list conversations: %w", err)
	}
	defer func() { _ = it.Close() }()

	var count int
	for {
		d, ok := it.Next(ctx)
		if !ok {
			break
		}
		fmt.Printf("%s\t%d messages\t%s\n", d.ID, d.MessageCount, d.UpdatedAt.Format("2006-01-02 15:04"))
		count++
	}
	if err := it.Err(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d conversations\n", count)
	return nil
}

// runCompile connects to the Rune workspace and runs the dream pipeline.
func (a *app) runCompile(
	ctx context.Context,
	source, output, model, apiKey, baseURL string,
	quiet, noAnimation bool,
) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	w, err := a.getWorkspace()
	if err != nil {
		return err
	}

	output, err = filepath.Abs(output)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}

	svc := w.LLM(ctx)
	entry, err := llmarg.Resolve(ctx, svc, model)
	if err != nil {
		return err
	}
	_ = apiKey
	_ = baseURL

	deps := dream.Deps{
		LLM:           svc,
		Store:         nil, // set by claudememory.Import
		Storage:       w.Storage(ctx),
		FS:            w.FileSystem(ctx),
		Exec:          w.Executor(ctx),
		LSP:           w.LSP(ctx),
		Parser:        w.Parser(ctx),
		Notifications: w.Notifications(ctx),
		DataPath:      output,
		Model:         entry,
	}

	it, err := claudememory.Import(ctx, source, deps)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	defer func() { _ = it.Close() }()

	if quiet {
		return drainSilent(ctx, it)
	}
	if noAnimation {
		return drainPlain(ctx, it)
	}
	return drainAnimated(ctx, it)
}

func drainAnimated(ctx context.Context, it iterator.Iterator[dream.Progress]) error {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetDescription("initializing"),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionShowCount(),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionClearOnFinish(),
	)

	var phase string
	for {
		p, ok := it.Next(ctx)
		if !ok {
			break
		}
		switch p.Type {
		case dream.ProgressBootstrap:
			phase = "bootstrapping"
			bar.Describe(p.Message)
		case dream.ProgressAnalyzing:
			phase = "compiling"
			bar.Describe(fmt.Sprintf("compiling %s", p.DialogueID))
			_ = bar.Add(1)
		case dream.ProgressWriting:
			phase = "extracting memories"
			bar.Describe(p.Message)
		case dream.ProgressToolCall:
			bar.Describe(fmt.Sprintf("%s: %s %s", phase, p.ToolName, p.Message))
		case dream.ProgressToolResult:
			if p.IsError {
				bar.Describe(fmt.Sprintf("%s: %s error", phase, p.ToolName))
			}
		case dream.ProgressVerifying:
			phase = "verifying"
			bar.Describe(p.Message)
		case dream.ProgressFixing:
			phase = "fixing"
			bar.Describe(p.Message)
		case dream.ProgressDone:
			_ = bar.Finish()
			fmt.Fprintf(os.Stderr, "%s\n", p.Message)
		}
	}
	if err := it.Err(); err != nil {
		_ = bar.Exit()
		return fmt.Errorf("dream: %w", err)
	}
	return nil
}

func drainSilent(ctx context.Context, it iterator.Iterator[dream.Progress]) error {
	for {
		_, ok := it.Next(ctx)
		if !ok {
			break
		}
	}
	if err := it.Err(); err != nil {
		return fmt.Errorf("dream: %w", err)
	}
	return nil
}

func drainPlain(ctx context.Context, it iterator.Iterator[dream.Progress]) error {
	var phase string
	for {
		p, ok := it.Next(ctx)
		if !ok {
			break
		}
		switch p.Type {
		case dream.ProgressBootstrap:
			phase = "bootstrapping"
			fmt.Fprintf(os.Stderr, "# %s\n", p.Message)
		case dream.ProgressAnalyzing:
			phase = "compiling"
			fmt.Fprintf(os.Stderr, "compile %s\n", p.DialogueID)
		case dream.ProgressToolCall:
			fmt.Fprintf(os.Stderr, "  %s: %s %s\n", phase, p.ToolName, p.Message)
		case dream.ProgressToolResult:
			if p.IsError {
				fmt.Fprintf(os.Stderr, "  %s: %s error\n", phase, p.ToolName)
			}
		case dream.ProgressVerifying:
			phase = "verifying"
			fmt.Fprintf(os.Stderr, "%s\n", p.Message)
		case dream.ProgressFixing:
			phase = "fixing"
			fmt.Fprintf(os.Stderr, "%s\n", p.Message)
		case dream.ProgressDone:
			fmt.Fprintf(os.Stderr, "# %s\n", p.Message)
		}
	}
	if err := it.Err(); err != nil {
		return fmt.Errorf("dream: %w", err)
	}
	return nil
}

func defaultClaudeHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}
