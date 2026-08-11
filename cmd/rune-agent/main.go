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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"unstable.build/go-tui/cmd/rune-agent/extension"
	"unstable.build/go-tui/cmd/rune-agent/headless"
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
	debug.StartPProfOnSignal()

	if slices.Contains(os.Args[1:], headlessFlag) {
		os.Exit(runHeadless(os.Args[1:]))
	}

	ext, metadata := extension.NewExtension()
	err := extensionapi.ServeWorkspaceExtension(ext, metadata)
	if err != nil {
		slog.Error("serve extension", "error", err)
		os.Exit(1)
	}
}

const headlessFlag = "--headless"

const headlessUsage = "usage: rune-agent --headless <instructions-file> " +
	"[--model <provider>/<model>] [--effort <effort>]"

// runHeadless drives a non-interactive run and returns the process exit
// code: 0 ran to completion, 1 could not start, 2 the agent loop
// terminated with an error, 130/143 interrupted by SIGINT/SIGTERM.
func runHeadless(args []string) int {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	opts, err := parseHeadlessArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rune-agent: %s\n", err)
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigch)

	// A runner timeout must still yield a partial trajectory, so a
	// signal only cancels the context and lets Run flush.
	var signalCode atomic.Int32
	go debug.CapturePanicReport(func() {
		select {
		case sig := <-sigch:
			if sig == syscall.SIGTERM {
				signalCode.Store(143)
			} else {
				signalCode.Store(130)
			}
			cancel()
		case <-ctx.Done():
		}
	})

	res, err := headless.Run(ctx, opts)
	if err != nil {
		slog.Error("headless run", "error", err)
		return 1
	}
	if code := signalCode.Load(); code != 0 {
		return int(code)
	}
	if res.AgentError != nil {
		slog.Error("agent loop", "error", res.AgentError)
		return 2
	}
	return 0
}

// parseHeadlessArgs parses the --headless invocation. The instructions
// file is mandatory and positional, immediately after --headless.
func parseHeadlessArgs(args []string) (headless.Options, error) {
	var opts headless.Options
	if len(args) < 2 || args[0] != headlessFlag {
		return opts, errors.New(headlessUsage)
	}
	opts.InstructionsFile = args[1]
	if strings.HasPrefix(opts.InstructionsFile, "-") {
		return headless.Options{}, errors.New(headlessUsage)
	}
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		var target *string
		switch rest[i] {
		case "--model":
			target = &opts.Model
		case "--effort":
			target = &opts.Effort
		default:
			return headless.Options{}, fmt.Errorf(
				"unknown argument %q\n%s", rest[i], headlessUsage)
		}
		if i+1 >= len(rest) {
			return headless.Options{}, fmt.Errorf(
				"%s requires a value\n%s", rest[i], headlessUsage)
		}
		*target = rest[i+1]
		i++
	}
	return opts, nil
}
