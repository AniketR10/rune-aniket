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

package agentshell

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"gopkg.in/yaml.v3"
)

// ConfigFS is the filesystem interface needed for config editing.
type ConfigFS interface {
	OpenFile(path string, flag int, perm os.FileMode) (workspaceapi.File, error)
	MkdirAll(path string, perm os.FileMode) error
}

func configFilePath(cwd workspaceapi.URI) string {
	return workspaceapi.Join(cwd, ".rune", "config.yaml").Path()
}

// AddSkillDir adds a skill directory to the skills list in .rune/config.yaml.
// If the config file doesn't exist, it creates the directory and file.
func AddSkillDir(fs ConfigFS, cwd workspaceapi.URI, dir string) error {
	path := configFilePath(cwd)
	root, err := readConfigNode(fs, path)
	if err != nil {
		return err
	}

	seq := findOrCreateSkillsSeq(root)

	// Check for duplicates.
	for _, n := range seq.Content {
		if n.Value == dir {
			return fmt.Errorf("directory already in config: %s", dir)
		}
	}

	seq.Content = append(seq.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: dir,
	})

	return writeConfigNode(fs, path, root)
}

// RemoveSkillDir removes a skill directory from the skills list in .rune/config.yaml.
// If the config file doesn't exist, it creates the directory and file.
func RemoveSkillDir(fs ConfigFS, cwd workspaceapi.URI, dir string) error {
	path := configFilePath(cwd)
	root, err := readConfigNode(fs, path)
	if err != nil {
		return err
	}

	seq := findOrCreateSkillsSeq(root)

	idx := -1
	for i, n := range seq.Content {
		if n.Value == dir {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("directory not in config: %s", dir)
	}

	seq.Content = append(seq.Content[:idx], seq.Content[idx+1:]...)
	return writeConfigNode(fs, path, root)
}

// SetMaxTokens writes the global max_tokens setting in .rune/config.yaml.
// If the config file doesn't exist, it creates the directory and file.
func SetMaxTokens(fs ConfigFS, cwd workspaceapi.URI, n int) error {
	path := configFilePath(cwd)
	root, err := readConfigNode(fs, path)
	if err != nil {
		return err
	}

	cfg := findOrCreateRuneAgentConfig(root)
	setScalarKey(cfg, "max_tokens", fmt.Sprint(n), "!!int")
	return writeConfigNode(fs, path, root)
}

func readConfigNode(fs ConfigFS, path string) (*yaml.Node, error) {
	f, err := fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// File doesn't exist — return a fresh empty document.
		return emptyDocNode(), nil
	}
	defer f.Close() //nolint:errcheck

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Empty file: treat as fresh.
	if doc.Kind == 0 {
		return emptyDocNode(), nil
	}
	return &doc, nil
}

func emptyDocNode() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		},
	}
}

func writeConfigNode(fs ConfigFS, path string, doc *yaml.Node) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("close encoder: %w", err)
	}

	// Ensure the parent directory exists.
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	wf, err := fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	defer wf.Close() //nolint:errcheck

	if _, err := wf.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// findOrCreateSkillsSeq navigates extensions.rune-agent.config.skills
// in the YAML node tree, creating missing intermediate nodes as needed.
// Returns the SequenceNode for skills.
func findOrCreateSkillsSeq(doc *yaml.Node) *yaml.Node {
	cfg := findOrCreateRuneAgentConfig(doc)
	return findOrCreateSeqKey(cfg, "skills")
}

func findOrCreateRuneAgentConfig(doc *yaml.Node) *yaml.Node {
	root := doc.Content[0] // document root mapping
	extensions := findOrCreateMapKey(root, "extensions")
	runeAgent := findOrCreateMapKey(extensions, "rune-agent")
	return findOrCreateMapKey(runeAgent, "config")
}

// findOrCreateMapKey finds or creates a mapping value for the given key
// in a MappingNode.
func findOrCreateMapKey(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	// Create key + empty mapping.
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content, keyNode, valNode)
	return valNode
}

// findOrCreateSeqKey finds or creates a sequence value for the given key
// in a MappingNode.
func findOrCreateSeqKey(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			return parent.Content[i+1]
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valNode := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	parent.Content = append(parent.Content, keyNode, valNode)
	return valNode
}

func setScalarKey(parent *yaml.Node, key, value, tag string) {
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			parent.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
	parent.Content = append(parent.Content, keyNode, valNode)
}
