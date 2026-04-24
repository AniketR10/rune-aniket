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


package markdown

import (
	"strings"
	"unicode"
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
			wordLen := len(word)

			if currentWidth > 0 && currentWidth+wordLen > width {
				lines = append(lines, currentLine)
				currentLine = nil
				currentWidth = 0
				word = strings.TrimLeft(word, " ")
				wordLen = len(word)
			}

			if wordLen > width {
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
					chunk := word
					if len(chunk) > remaining {
						chunk = word[:remaining]
					}
					currentLine = append(currentLine, span{
						text:  chunk,
						style: sp.style,
						url:   sp.url,
					})
					currentWidth += len(chunk)
					word = word[len(chunk):]
				}
			} else if wordLen > 0 {
				currentLine = append(currentLine, span{
					text:  word,
					style: sp.style,
					url:   sp.url,
				})
				currentWidth += wordLen
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
