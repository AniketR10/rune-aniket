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

package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/workspace/walkdir"
)

// contextCompleter backs the chat's '#' completion band with workspace
// files followed by workspace symbols.
type contextCompleter struct {
	fs      workspaceapi.FileSystem
	parser  syntaxapi.Parser
	ignore  walkdir.Filter
	protect walkdir.Filter
}

func newContextCompleter(
	fs workspaceapi.FileSystem, parser syntaxapi.Parser,
) *contextCompleter {
	c := &contextCompleter{fs: fs, parser: parser}
	// Hidden entries are pruned at the walk source rather than after the
	// fact: descending into trees like ~/.cache floods the filesystem with
	// ReadDir calls for candidates that are never useful as attachments.
	protect := vctrl.AnyMatcher(
		vctrl.ProtectedDirMatcher(fs), vctrl.HiddenBaseMatcher())
	c.protect = protect
	// Match the IDE fuzzy finder and the agent's file-walking tools;
	// degrade to hidden/protected filtering alone on schemes without a
	// gitignore.
	c.ignore = protect
	if m, err := vctrl.LoadGitignore(fs); err != nil {
		slog.Warn("extension: load gitignore matcher", "error", err)
	} else {
		c.ignore = vctrl.AnyMatcher(m, protect)
	}
	return c
}

// Candidates merges the file and symbol sources into a single stream of
// icon-prefixed entries. Both sources are consumed here so their
// differing iterator types never reach dialoguetui.
func (c *contextCompleter) Candidates(
	ctx context.Context, query string,
) (iterator.Iterator[string], error) {
	ctx, cancel := context.WithCancel(ctx)
	out := make(chan string)
	done := make(chan struct{})
	go debug.CapturePanicReport(func() {
		defer close(done)
		defer close(out)
		var wg sync.WaitGroup
		wg.Add(2)
		go debug.CapturePanicReport(func() {
			defer wg.Done()
			c.streamFiles(ctx, out, query)
		})
		go debug.CapturePanicReport(func() {
			defer wg.Done()
			c.streamSymbols(ctx, out)
		})
		wg.Wait()
	})
	return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
		select {
		case v, ok := <-out:
			return v, ok, nil
		case <-ctx.Done():
			return "", false, ctx.Err()
		}
	}, func() error {
		cancel()
		<-done
		return nil
	}), nil
}

func (c *contextCompleter) streamFiles(
	ctx context.Context, out chan<- string, query string,
) {
	root, display, filter, err := c.fileRoot(query)
	if err != nil {
		slog.Warn("extension: resolve context completion root", "error", err)
		return
	}
	it, err := walkdir.ListFiles(
		walkdir.WithContextFilter(ctx, filter), c.fs, root)
	if err != nil {
		slog.Warn("extension: list workspace files", "error", err)
		return
	}
	defer func() { _ = it.Close() }()
	prefix := string(dialoguetui.WorkspaceFileIcon) + " "
	for {
		p, ok := it.Next(ctx)
		if !ok {
			return
		}
		select {
		case out <- prefix + display(p):
		case <-ctx.Done():
			return
		}
	}
}

func (c *contextCompleter) fileRoot(
	query string,
) (string, func(string) string, walkdir.Filter, error) {
	root := query
	if root == "" {
		root = "."
	}
	cwd, err := c.fs.URI(".")
	if err != nil {
		return "", nil, nil, err
	}
	target, err := c.fs.URI(root)
	if err != nil {
		return "", nil, nil, err
	}
	filter := c.ignore
	if !sameWorkspaceTree(cwd, target) {
		filter = c.protect
	}
	dotRelative := query == "." || strings.HasPrefix(query, "./")
	display := func(path string) string {
		if filepath.IsAbs(query) {
			if filepath.IsAbs(path) {
				return path
			}
			return filepath.Join(cwd.Path(), path)
		}
		if !filepath.IsAbs(path) {
			if dotRelative && !strings.HasPrefix(path, "."+string(filepath.Separator)) {
				return "." + string(filepath.Separator) + path
			}
			return path
		}
		rel, err := filepath.Rel(cwd.Path(), path)
		if err != nil {
			return path
		}
		if dotRelative && !strings.HasPrefix(rel, "."+string(filepath.Separator)) {
			return "." + string(filepath.Separator) + rel
		}
		return rel
	}
	if query == "~" || query == "/~" || strings.HasPrefix(query, "~/") ||
		strings.HasPrefix(query, "/~/") {
		home, err := c.fs.URI("~")
		if err != nil {
			return "", nil, nil, err
		}
		homePrefix := "~"
		if query == "/~" || strings.HasPrefix(query, "/~/") {
			homePrefix = string(filepath.Separator) + "~"
		}
		display = func(path string) string {
			absolute := path
			if !filepath.IsAbs(absolute) {
				absolute = filepath.Join(cwd.Path(), absolute)
			}
			rel, err := filepath.Rel(home.Path(), absolute)
			if err != nil || rel == ".." ||
				strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return path
			}
			return filepath.Join(homePrefix, rel)
		}
	}
	return root, display, filter, nil
}

func sameWorkspaceTree(base, target workspaceapi.URI) bool {
	if base.Scheme() != target.Scheme() || base.Host() != target.Host() ||
		base.User() != target.User() {
		return false
	}
	rel, err := filepath.Rel(base.Path(), target.Path())
	return err == nil && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (c *contextCompleter) streamSymbols(ctx context.Context, out chan<- string) {
	it, err := referencedSymbols(ctx, c.parser)
	if err != nil {
		slog.Warn("extension: list referenced symbols", "error", err)
		return
	}
	defer func() { _ = it.Close() }()
	prefix := string(dialoguetui.SymbolIcon) + " "
	for {
		s, ok := it.Next(ctx)
		if !ok {
			return
		}
		select {
		case out <- prefix + s:
		case <-ctx.Done():
			return
		}
	}
}

func referencedSymbols(
	ctx context.Context, parser syntaxapi.Parser,
) (iterator.Iterator[string], error) {
	it, err := parser.ListReferencedSymbols(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
		for {
			s, ok := it.Next(ctx)
			if !ok {
				return "", false, it.Err()
			}
			if seen[s] {
				continue
			}
			seen[s] = true
			return s, true, nil
		}
	}, it.Close), nil
}

// Resolve strips the icon prefix Candidates added and builds the
// attachment the entry stands for.
func (c *contextCompleter) Resolve(
	candidate string,
) (dialoguetui.Attachment, bool) {
	icon, size := utf8.DecodeRuneInString(candidate)
	if size == 0 || len(candidate) <= size || candidate[size] != ' ' {
		return dialoguetui.Attachment{}, false
	}
	value := candidate[size+1:]
	switch icon {
	case dialoguetui.WorkspaceFileIcon:
		return dialoguetui.NewWorkspaceFileAttachment(value), true
	case dialoguetui.SymbolIcon:
		return dialoguetui.NewSymbolAttachment(value), true
	}
	return dialoguetui.Attachment{}, false
}

// symbolSections are the tools whose combined output stands in for a
// symbol attachment, in the order they are rendered. Reusing the
// registered tools inherits their match caps and hints.
var symbolSections = []struct {
	tool    string
	heading string
	// Location tools emit one result per line, which markdown would
	// otherwise reflow into a single unreadable paragraph.
	locations bool
}{
	{"find_definition", "## Definition", true},
	{"find_references", "## References", true},
	{"describe_symbol", "## Documentation", false},
}

// symbolContext expands a symbol attachment into the definition,
// reference and documentation context the agent would otherwise have to
// gather with three tool calls.
func (h *aiEditorHandler) symbolContext(ctx context.Context, symbol string) string {
	args, err := json.Marshal(map[string]string{"symbol": symbol})
	if err != nil {
		return fmt.Sprintf("could not build symbol query: %v", err)
	}
	var b strings.Builder
	for _, s := range symbolSections {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s.heading)
		b.WriteByte('\n')
		tool, ok := h.toolRegistry.Get(s.tool, "")
		if !ok {
			fmt.Fprintf(&b, "(%s is unavailable)", s.tool)
			continue
		}
		res := tool.Execute(ctx, string(args))
		if res.IsError {
			fmt.Fprintf(&b, "(%s failed: %s)", s.tool, res.Content)
			continue
		}
		if s.locations {
			b.WriteString(locationList(res.Content))
			continue
		}
		b.WriteString(res.Content)
	}
	return b.String()
}

// locationList renders one location per markdown list item. Locations
// quote source lines, so each is wrapped in a code span to stop
// markdown from reading underscores and asterisks in identifiers as
// emphasis.
func locationList(content string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("- ")
		b.WriteString(codeSpan(line))
	}
	return b.String()
}

// codeSpan wraps s in a backtick run longer than any run it contains,
// which is how CommonMark lets a code span hold literal backticks.
func codeSpan(s string) string {
	longest, run := 0, 0
	for _, r := range s {
		if r != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	fence := strings.Repeat("`", longest+1)
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		return fence + " " + s + " " + fence
	}
	return fence + s + fence
}
