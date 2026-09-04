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

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// Grid sizes in pixels for the two benchmark resolutions. Device
// scale is pinned to 1 so these are also the device-pixel dimensions.
const (
	benchHDWidth, benchHDHeight = 1920, 1080
	bench4KWidth, bench4KHeight = 3840, 2160
)

// benchFixtureGo is a syntax-highlighted Go source fixture large
// enough to fill a 4K viewport when scrolled. It is opened as a plain
// text buffer with the committed go tree-sitter grammar staged, so
// highlighting engages without a language server.
var benchFixtureGo = strings.Repeat(
	`package sample

import (
	"context"
	"fmt"
	"sort"
)

// Widget models a scored, tagged record used by the renderer battery.
type Widget struct {
	ID    int
	Name  string
	Score float64
	Tags  []string
}

func (w Widget) Total(base float64) float64 {
	total := base
	for range w.Tags {
		total += w.Score * 1.5
	}
	return total
}

func Process(ctx context.Context, items []Widget) (map[string]float64, error) {
	out := make(map[string]float64, len(items))
	sort.Slice(items, func(i, j int) bool { return items[i].Score > items[j].Score })
	for _, it := range items {
		out[it.Name] = it.Total(float64(it.ID))
	}
	fmt.Printf("processed %d widgets\n", len(out))
	return out, nil
}

`, 24)

// benchFixtureUnicode stresses wide cells: CJK, emoji, box-drawing and
// braille, all of which route through the custom / fallback font faces.
var benchFixtureUnicode = strings.Repeat(
	"日本語のテキスト 中文字符 한국어 😀🚀🎉🔥✨ "+
		"┌──┬──┐ │ab│cd│ ╞══╪══╡ ⠁⠂⠃⠄⠅⠆⠇⣿ ▁▂▃▄▅▆▇█\n", 200)

// buildWorstCaseFixture fills every cell with a distinct glyph so the
// worst-case scenario defeats background-skip and damage tracking.
func buildWorstCaseFixture() string {
	var b strings.Builder
	rs := []rune("▚▞█▓▒░◢◣◤◥●◐◑◒◓★☆♦♣♠♥")
	for line := range 400 {
		for col := range 220 {
			b.WriteRune(rs[(line*7+col*13)%len(rs)])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// dispatch runs an ex command on the focused window by typing it into
// the command prompt through the production key path, then pumps a few
// frames so the command executes and redraws.
func (s *guiBenchSession) dispatch(cmd string, args ...string) {
	joined := cmd
	if len(args) > 0 {
		joined = cmd + " " + strings.Join(args, " ")
	}
	s.publishKeys(":" + joined)
	s.publish(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	for range 20 {
		s.frame()
	}
}

// openWorkspaceFile opens and focuses a workspace-relative file through
// the production :edit command so subsequent keystrokes drive a live
// editor buffer.
func (s *guiBenchSession) openWorkspaceFile(rel string) {
	s.dispatch("edit", filepath.Join(s.workDir, filepath.FromSlash(rel)))
}

// scrollStep scrolls the viewport by one line every frame, reversing
// direction periodically so the buffer never runs out. Moving each
// frame shifts the whole viewport, producing genuine full-viewport
// damage (the case row-damage tracking must not slow down).
func scrollStep(s *guiBenchSession, frame int) {
	if (frame/40)%2 == 0 {
		s.publish(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
	} else {
		s.publish(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
	}
}

// pageScrollStep pages the viewport down/up every frame, so the whole
// viewport turns over each frame. This is the full-viewport-damage
// stress that row-damage tracking must not regress.
func pageScrollStep(s *guiBenchSession, frame int) {
	if (frame/20)%2 == 0 {
		s.publish(term.Event{Type: term.EventKey, Key: term.KeyPgdn})
	} else {
		s.publish(term.Event{Type: term.EventKey, Key: term.KeyPgup})
	}
}

func idleScenario() guiBenchScenario { return guiBenchScenario{name: "idle"} }

func cursorMoveScenario() guiBenchScenario {
	return guiBenchScenario{
		name:           "cursor-move",
		workspaceFiles: map[string]string{"main.go": benchFixtureGo},
		syntaxLangs:    []string{"go"},
		openFile:       "main.go",
		step: func(s *guiBenchSession, frame int) {
			if frame%2 == 0 {
				s.publish(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
			} else {
				s.publish(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
			}
		},
	}
}

func typingScenario() guiBenchScenario {
	line := []rune("the quick brown fox jumps over the lazy dog ")
	return guiBenchScenario{
		name:           "typing",
		workspaceFiles: map[string]string{"main.go": benchFixtureGo},
		syntaxLangs:    []string{"go"},
		openFile:       "main.go",
		setup: func(s *guiBenchSession) {
			// Open a new line in insert mode so keystrokes append text
			// instead of overwriting the fixture in place.
			s.publish(term.Event{Type: term.EventKey, Ch: 'o'})
			s.frame()
		},
		step: func(s *guiBenchSession, frame int) {
			s.publish(term.Event{Type: term.EventKey, Ch: line[frame%len(line)]})
		},
	}
}

func scrollScenario() guiBenchScenario {
	return guiBenchScenario{
		name:           "scroll",
		workspaceFiles: map[string]string{"main.go": benchFixtureGo},
		syntaxLangs:    []string{"go"},
		openFile:       "main.go",
		step:           scrollStep,
	}
}

func splitsScenario() guiBenchScenario {
	return guiBenchScenario{
		name: "splits",
		workspaceFiles: map[string]string{
			"main.go":  benchFixtureGo,
			"notes.md": strings.Repeat("# Notes\n\nsome prose paragraph here.\n\n", 60),
		},
		syntaxLangs: []string{"go"},
		openFile:    "main.go",
		setup: func(s *guiBenchSession) {
			s.dispatch("windownew", "right")
			s.dispatch("windownew", "down")
		},
		step: scrollStep,
	}
}

func selectionDragScenario() guiBenchScenario {
	return guiBenchScenario{
		name:           "selection-drag",
		workspaceFiles: map[string]string{"main.go": benchFixtureGo},
		syntaxLangs:    []string{"go"},
		openFile:       "main.go",
		step: func(s *guiBenchSession, frame int) {
			x := 2 + frame%60
			y := 2 + (frame/60)%20
			s.publish(term.Event{
				Type: term.EventMouse, Key: term.MouseLeft,
				MouseX: x, MouseY: y,
			})
		},
	}
}

func invalidateBurstScenario() guiBenchScenario {
	themes := []string{"romero", "carmack", "mullen", "hopper"}
	return guiBenchScenario{
		name:           "invalidate-burst",
		workspaceFiles: map[string]string{"main.go": benchFixtureGo},
		syntaxLangs:    []string{"go"},
		openFile:       "main.go",
		step: func(s *guiBenchSession, frame int) {
			s.dispatch("guitheme", themes[frame%len(themes)])
		},
	}
}

func unicodeStressScenario() guiBenchScenario {
	return guiBenchScenario{
		name:           "unicode-stress",
		workspaceFiles: map[string]string{"unicode.txt": benchFixtureUnicode},
		openFile:       "unicode.txt",
		step:           scrollStep,
	}
}

func worstCaseScenario() guiBenchScenario {
	return guiBenchScenario{
		name:           "worst-case",
		workspaceFiles: map[string]string{"noise.txt": buildWorstCaseFixture()},
		openFile:       "noise.txt",
		step:           pageScrollStep,
	}
}

// guiBenchScenarios is the battery. Each is run at HD and 4K, opaque
// and transparent, by BenchmarkGUI.
func guiBenchScenarios() []guiBenchScenario {
	return []guiBenchScenario{
		idleScenario(),
		cursorMoveScenario(),
		typingScenario(),
		scrollScenario(),
		splitsScenario(),
		selectionDragScenario(),
		invalidateBurstScenario(),
		unicodeStressScenario(),
		worstCaseScenario(),
	}
}

// benchResolutions enumerates the two measured grid sizes.
var benchResolutions = []struct {
	name string
	w, h int
}{
	{"HD", benchHDWidth, benchHDHeight},
	{"4K", bench4KWidth, bench4KHeight},
}

// repaintModes enumerates the innermost benchmark axis: the production
// damage-tracked renderer versus the reference full-repaint renderer.
// Pairing them under a stable name lets benchstat quantify the win from
// row-damage tracking directly, in one binary on one GPU, e.g.
// BenchmarkGUI/scroll/HD/opaque/optimized vs .../reference.
var repaintModes = []struct {
	name             string
	forceFullRepaint bool
}{
	{"optimized", false},
	{"reference", true},
}

// BenchmarkGUI runs the full scenario battery across both resolutions,
// the opaque / transparent background variants, and the optimized /
// reference repaint modes. Sub-benchmark names are stable so benchstat
// can compare runs across commits, e.g. BenchmarkGUI/scroll/HD/opaque.
func BenchmarkGUI(b *testing.B) {
	for _, sc := range guiBenchScenarios() {
		b.Run(sc.name, func(b *testing.B) {
			for _, res := range benchResolutions {
				b.Run(res.name, func(b *testing.B) {
					for _, transparent := range []bool{false, true} {
						variant := "opaque"
						if transparent {
							variant = "transparent"
						}
						b.Run(variant, func(b *testing.B) {
							for _, mode := range repaintModes {
								b.Run(mode.name, func(b *testing.B) {
									runGUIBenchScenario(b, sc, guiBenchConfig{
										pixelsW:          res.w,
										pixelsH:          res.h,
										transparent:      transparent,
										forceFullRepaint: mode.forceFullRepaint,
									})
								})
							}
						})
					}
				})
			}
		})
	}
}
