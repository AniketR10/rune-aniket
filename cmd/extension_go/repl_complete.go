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

package main

import (
	"context"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

const completionLanguageID = "go"

// CompletionPrefix returns the trailing Go identifier the completion
// overlay must replace on accept. ideshell deletes this many runes
// before inserting the chosen candidate, so it has to be just the
// partial member name at the cursor (e.g. "fmt.Pri" -> "Pri"), not the
// whole whitespace-delimited token. A line ending in a non-identifier
// byte (e.g. "fmt.") yields an empty prefix, so the candidate is
// inserted as-is.
func (s *goSession) CompletionPrefix(line string) string {
	if prefix, ok := importPathPrefix(line); ok {
		return prefix
	}
	// A builtin like /help is a single token; the candidate replaces it
	// whole, so the leading sigil must be part of the prefix.
	if strings.HasPrefix(line, builtinPrefix) && !strings.ContainsAny(line, " \t") {
		return line
	}
	i := len(line)
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:i])
		if !isIdentRune(r) {
			break
		}
		i -= size
	}
	return line[i:]
}

// importPathPrefix reports whether line is a bare import whose path is
// being typed, returning the partial path already entered inside the
// opening quote. It matches lines like `import "fm` or
// `import "github.com/`, the only place package-path completion applies.
// An alias (`import w "io`) is allowed; only the text after the last
// quote is the prefix.
func importPathPrefix(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "import") {
		return "", false
	}
	rest := trimmed[len("import"):]
	if rest != "" && !isSpace(rest[0]) {
		return "", false
	}
	// An odd number of quotes means the final one opened a string that is
	// still being typed (the partial path); an even count means every
	// path is closed, so there is nothing left to complete.
	if strings.Count(rest, `"`)%2 == 0 {
		return "", false
	}
	q := strings.LastIndexByte(rest, '"')
	return rest[q+1:], true
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

// rejoinArgs reconstructs the raw Go fragment from the cmd/args split
// ideshell performs on whitespace. Completion needs the full line so the
// fragment ("buf.Wri") survives intact through gopls.
func rejoinArgs(cmd string, args []string) string {
	if len(args) == 0 {
		return cmd
	}
	return cmd + " " + strings.Join(args, " ")
}

// completeGo asks gopls to complete a partial Go fragment. It renders an
// accumulated program whose final line is the fragment, writes it to the
// shared program file, opens it as an LSP overlay, and requests
// completion at the cursor sitting just past the fragment. Candidates are
// the bare insert texts that replace the trailing identifier (see
// CompletionPrefix); the caller's overlay restores the text before it.
func (s *goSession) completeGo(
	ctx context.Context, fragment string,
) (iterator.Iterator[string], error) {
	if s.lsp == nil {
		return iterator.Empty[string](), nil
	}
	cr, ok := s.runner.(completionRunner)
	if !ok {
		return iterator.Empty[string](), nil
	}
	src, offset := s.renderForCompletion(fragment)
	path, err := cr.programPath(src)
	if err != nil {
		return iterator.Empty[string](), nil
	}
	uri, err := s.fs.URI(path)
	if err != nil {
		return iterator.Empty[string](), nil
	}
	lspURI := "file://" + uri.Path()
	pos := byteOffsetToPosition(src, offset)

	if err := s.lsp.DidOpen(ctx, semanticapi.DidOpenTextDocumentParams{
		TextDocument: semanticapi.TextDocumentItem{
			URI:        lspURI,
			LanguageID: completionLanguageID,
			Version:    1,
			Text:       src,
		},
	}); err != nil {
		return iterator.Empty[string](), nil
	}
	defer func() {
		_ = s.lsp.DidClose(ctx, semanticapi.DidCloseTextDocumentParams{
			TextDocument: semanticapi.TextDocumentIdentifier{URI: lspURI},
		})
	}()

	result, err := s.lsp.Completion(ctx, semanticapi.CompletionParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: lspURI},
		Position:     pos,
		Context: &semanticapi.CompletionContext{
			TriggerKind: semanticapi.CompletionTriggerKindInvoked,
		},
	})
	if err != nil {
		return iterator.Empty[string](), nil
	}
	return iterator.FromSlice(completionInserts(result.Items)), nil
}

// completionInserts maps gopls completion items to the bare text that
// replaces the trailing identifier. gopls fills InsertText/TextEdit with
// the member name; Label is the fallback. Duplicate and empty inserts are
// dropped so the overlay shows each candidate once.
func completionInserts(items []semanticapi.CompletionItem) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		text := insertText(item)
		if text == "" {
			continue
		}
		if _, dup := seen[text]; dup {
			continue
		}
		seen[text] = struct{}{}
		out = append(out, text)
	}
	return out
}

// insertText extracts the text a completion item inserts, preferring the
// explicit edit text, then InsertText, then Label. A TextEdit's NewText
// can carry snippet placeholders or a leading prefix; the overlay inserts
// it verbatim, so the bare identifier forms gopls returns for member
// completion map cleanly.
func insertText(item semanticapi.CompletionItem) string {
	switch {
	case item.TextEdit != nil && item.TextEdit.NewText != "":
		return item.TextEdit.NewText
	case item.InsertText != "":
		return item.InsertText
	default:
		return item.Label
	}
}

// renderForCompletion builds a complete Go program whose final line is
// the partial fragment, returning the source and the byte offset of the
// cursor sitting immediately after the fragment. Unlike render it does
// not gofmt the result: the fragment is usually unfinished (e.g.
// "fmt.Pri"), which gofmt would reject, and gofmt could also shift the
// cursor. gopls completes unfinished code, so raw source is what it
// needs. Imports that the fragment references stay normal imports (not
// blank) so their members resolve.
func (s *goSession) renderForCompletion(fragment string) (string, int) {
	var b strings.Builder
	b.WriteString("package main\n\n")

	specs := s.sortedImports()
	if len(specs) > 0 {
		body := s.importBody(fragment)
		b.WriteString("import (\n")
		for _, spec := range specs {
			b.WriteString("\t" + renderImport(spec, body) + "\n")
		}
		b.WriteString(")\n\n")
	}

	for _, decl := range s.decls {
		b.WriteString(decl)
		b.WriteString("\n\n")
	}

	b.WriteString("func main() {\n")
	for _, stmt := range s.stmts {
		b.WriteString(stmt + "\n")
	}
	for _, name := range s.declared {
		b.WriteString("\t_ = " + name + "\n")
	}
	b.WriteString("\t")
	b.WriteString(fragment)
	offset := b.Len()
	b.WriteString("\n}\n")
	return b.String(), offset
}

// byteOffsetToPosition converts a byte offset in src to an LSP Position
// (0-based line, UTF-16 code-unit character), as gopls expects.
func byteOffsetToPosition(src string, offset int) semanticapi.Position {
	if offset > len(src) {
		offset = len(src)
	}
	line := 0
	lineStart := 0
	for i := 0; i < offset; i++ {
		if src[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	character := utf16Len(src[lineStart:offset])
	return semanticapi.Position{
		Line:      uint32(line),
		Character: uint32(character),
	}
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += len(utf16.Encode([]rune{r}))
	}
	return n
}

// isIdentRune reports whether r can appear in a Go identifier. Selectors
// (".") are deliberately excluded so the completion prefix is only the
// trailing member name.
func isIdentRune(r rune) bool {
	return r == '_' ||
		(r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9')
}
