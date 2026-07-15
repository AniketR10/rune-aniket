// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

// Command runesvc is a stripped-down stand-in for the `rune` workspace
// server (see cmd/rune/main.go's --workspace-server / -x flag).
//
// It exists solely to give the SSH e2e tests a Linux binary they can
// install as `rune` inside the test container, exercising the full
// connectScheme path (whichCommand + workspaceExists + StartSchemeServer)
// without having to cross-compile the full rune binary (which depends on
// CGO and GUI libraries that don't cross-compile cleanly from a dev
// laptop).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"gopkg.in/yaml.v3"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/workspacerpc"
	"unstable.build/go-tui/workspace/workspacessh"
)

func main() {
	workspacePath := flag.String("x", "",
		"local workspace path to expose over the workspace gRPC server")
	install := flag.String("install", "",
		"CSV manifest of id@version packages to provision (mirrors the "+
			"real rune -x flag). When non-empty, runesvc loads "+
			"~/.rune/config.yaml and applies its gui.env block to this "+
			"process before serving, exercising the provisioning ordering.")
	flag.Parse()

	if *workspacePath == "" {
		fmt.Fprintln(os.Stderr, "runesvc: -x <workspace-path> is required")
		os.Exit(2)
	}

	// Mirror the real -x path's ordering: provisioning (install + config
	// load + gui.env apply) happens before the server starts serving, so a
	// child process spawned over the workspace RPC inherits the applied env.
	if *install != "" {
		emitProvisionProgress(*install)
		installPackageBins(*install)
		applyRemoteGUIEnv()
	}

	uri, err := workspaceapi.ParseURI("file://" + *workspacePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runesvc: parse uri:", err)
		os.Exit(3)
	}
	scheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), uri)
	if err != nil {
		fmt.Fprintln(os.Stderr, "runesvc: new file scheme:", err)
		os.Exit(4)
	}
	defer scheme.Close()

	server := workspacerpc.NewServer(scheme, new(sync.Mutex),
		workspacerpc.CommandAuthorizerFunc(
			func(context.Context, workspaceapi.Cmd) error { return nil }))
	defer func() { _ = server.Stop() }()

	if err := workspacessh.StartSchemeServer(
		log.New(), server, workspacessh.NewSchemeServer()); err != nil {
		fmt.Fprintln(os.Stderr, "runesvc: start scheme server:", err)
		os.Exit(5)
	}
}

// applyRemoteGUIEnv loads ~/.rune/config.yaml and applies its gui.env block to
// this process, standing in for the real -x server's post-install config load.
// Failures warn and continue, matching the never-abort provisioning policy.
func applyRemoteGUIEnv() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		if u, uerr := user.Current(); uerr == nil {
			home = u.HomeDir
		}
	}
	if home == "" {
		fmt.Fprintln(os.Stderr, "runesvc: cannot resolve home dir")
		return
	}
	configPath := filepath.Join(home, ".rune", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runesvc: read %s: %v\n", configPath, err)
		return
	}
	var doc struct {
		GUI struct {
			Env map[string]any `yaml:"env"`
		} `yaml:"gui"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "runesvc: parse %s: %v\n", configPath, err)
		return
	}
	for k, v := range doc.GUI.Env {
		if err := os.Setenv(k, fmt.Sprintf("%v", v)); err != nil {
			fmt.Fprintf(os.Stderr, "runesvc: setenv %s: %v\n", k, err)
		}
	}
}

// emitProvisionProgress streams JSON-Lines provisioning progress to stderr for
// each package in the manifest, standing in for the real -x server's install
// loop. The local side (workspacessh) forwards each line to a UI notification,
// so the e2e test can assert the ordered stream reaches the browser.
func emitProvisionProgress(manifest string) {
	entries := strings.Split(manifest, ",")
	total := len(entries)
	for i, entry := range entries {
		id, ver, _ := strings.Cut(entry, "@")
		index := i + 1
		emit(workspacessh.ProvisionProgress{
			Index: index, Total: total, Package: id, Version: ver,
			Phase: workspacessh.ProvisionPhaseInstalling,
		})
		emit(workspacessh.ProvisionProgress{
			Index: index, Total: total, Package: id, Version: ver,
			Phase: workspacessh.ProvisionPhaseActivating,
		})
	}
	emit(workspacessh.ProvisionProgress{
		Index: total, Total: total, Phase: workspacessh.ProvisionPhaseDone,
	})
}

func emit(p workspacessh.ProvisionProgress) {
	line, err := workspacessh.EncodeProvisionProgress(p)
	if err != nil {
		return
	}
	fmt.Fprint(os.Stderr, line)
}

// installPackageBins stands in for a real package install by writing an
// executable for each manifest entry into ~/.rune/bin and prepending that
// directory to PATH, exactly as the real -x server does via setupRuneBinPATH.
// Each fake tool is named after the package id and prints a recognizable line
// so an e2e test can run it through the workspace executor and prove the
// provisioned toolchain is on the served process's PATH.
func installPackageBins(manifest string) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		if u, uerr := user.Current(); uerr == nil {
			home = u.HomeDir
		}
	}
	if home == "" {
		fmt.Fprintln(os.Stderr, "runesvc: cannot resolve home dir for bin install")
		return
	}
	binDir := filepath.Join(home, ".rune", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "runesvc: mkdir %s: %v\n", binDir, err)
		return
	}
	for entry := range strings.SplitSeq(manifest, ",") {
		id, ver, _ := strings.Cut(entry, "@")
		if id == "" {
			continue
		}
		toolPath := filepath.Join(binDir, id)
		script := fmt.Sprintf("#!/bin/sh\necho \"%s ok %s\"\n", id, ver)
		if err := os.WriteFile(toolPath, []byte(script), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "runesvc: write %s: %v\n", toolPath, err)
			continue
		}
	}
	if err := os.Setenv("PATH", binDir+":"+os.Getenv("PATH")); err != nil {
		fmt.Fprintf(os.Stderr, "runesvc: set PATH: %v\n", err)
	}
}
