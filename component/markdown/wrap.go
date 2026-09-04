// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package markdown

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

// wrapTextRun wraps a textRun to fit within the given width.
// Returns the wrapped lines as a slice of textRuns.
func wrapTextRun(run textRun, width int) []textRun {
	if width <= 0 {
		return nil
	}

	var lines []textRun
	var currentLine textRun
	var currentWidth int
	var needSpace bool

	for _, sp := range run {
		words, hasLeadingSpace, hasTrailingSpace := splitIntoWordsWithSpaces(sp.text)

		if hasLeadingSpace && len(words) == 0 {
			needSpace = true
			continue
		}

		for i, word := range words {
			// Add space as a separate unstyled span so it doesn't inherit styled
			// attributes (like inline code background or link underline)
			if (needSpace || (hasLeadingSpace && i == 0)) && currentWidth > 0 {
				currentLine = append(currentLine, span{text: " ", style: styleNone})
				currentWidth++
			}
			wordWidth := textWidth(word)

			if currentWidth > 0 && currentWidth+wordWidth > width {
				lines = append(lines, currentLine)
				currentLine = nil
				currentWidth = 0
				word = strings.TrimLeft(word, " ")
				wordWidth = textWidth(word)
			}

			if wordWidth > width {
				for len(word) > 0 {
					remaining := width - currentWidth
					if remaining <= 0 {
						if len(currentLine) > 0 {
							lines = append(lines, currentLine)
						}
						currentLine = nil
						currentWidth = 0
						remaining = width
					}
					chunk, chunkWidth := clusterChunk(word, remaining)
					if chunk == "" && currentWidth > 0 {
						// The next cluster is wider than what is left of this
						// line; restart it on the next one.
						lines = append(lines, currentLine)
						currentLine = nil
						currentWidth = 0
						continue
					}
					if chunk == "" {
						// Wider than the whole line: place it anyway so
						// wrapping always makes progress. Draw clips it.
						chunk, chunkWidth = firstCluster(word)
					}
					currentLine = append(currentLine, span{
						text:  chunk,
						style: sp.style,
						url:   sp.url,
					})
					currentWidth += chunkWidth
					word = word[len(chunk):]
				}
			} else if wordWidth > 0 {
				currentLine = append(currentLine, span{
					text:  word,
					style: sp.style,
					url:   sp.url,
				})
				currentWidth += wordWidth
			}
			needSpace = false
		}
		if hasTrailingSpace {
			needSpace = true
		}
	}

	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}

	return lines
}

// clusterChunk returns the longest prefix of word that fits in maxWidth
// columns without splitting a grapheme cluster. It returns an empty chunk
// when the leading cluster alone is wider than maxWidth.
func clusterChunk(word string, maxWidth int) (chunk string, chunkWidth int) {
	rest := word
	state := -1
	var width uint8
	for len(rest) > 0 {
		// An ASCII byte whose successor is also ASCII (or end of string) is a
		// complete width-1 cluster: combining marks are never ASCII.
		if rest[0] < utf8.RuneSelf &&
			(len(rest) == 1 || rest[1] < utf8.RuneSelf) {
			if chunkWidth >= maxWidth {
				break
			}
			rest = rest[1:]
			chunk = word[:len(word)-len(rest)]
			chunkWidth++
			state = -1
			continue
		}
		_, rest, width, state = graphemecluster.StepString(rest, state)
		cols := max(1, int(width))
		if chunkWidth+cols > maxWidth {
			break
		}
		chunk = word[:len(word)-len(rest)]
		chunkWidth += cols
	}
	return
}

func firstCluster(text string) (cluster string, width int) {
	c, _, w, _ := graphemecluster.StepString(text, -1)
	return c, max(1, int(w))
}

// splitIntoWordsWithSpaces splits text into words, preserving spaces as part
// of the following word when they occur between words. Returns the words,
// whether there was a leading space, and whether there was a trailing space.
func splitIntoWordsWithSpaces(text string) (words []string, leadingSpace, trailingSpace bool) {
	var current strings.Builder
	var sawSpace bool
	var sawNonSpace bool

	for _, r := range text {
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			sawSpace = true
			if !sawNonSpace {
				leadingSpace = true
			}
		} else {
			if sawSpace && sawNonSpace {
				current.WriteRune(' ')
			}
			sawSpace = false
			sawNonSpace = true
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	trailingSpace = sawSpace && sawNonSpace
	if !sawNonSpace && sawSpace {
		leadingSpace = true
		trailingSpace = true
	}
	return
}

// countWrappedLines returns the number of lines needed to display the run.
func countWrappedLines(run textRun, width int) int {
	if width <= 0 {
		return 0
	}
	lines := wrapTextRun(run, width)
	if len(lines) == 0 {
		return 1
	}
	return len(lines)
}
