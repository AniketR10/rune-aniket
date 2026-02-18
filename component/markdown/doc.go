// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

// Package markdown provides a terminal-based markdown rendering component.
//
// The component parses markdown content using goldmark and renders it
// with proper formatting for terminal display, supporting headers, paragraphs,
// lists (ordered, unordered, task), code blocks, blockquotes, tables,
// and horizontal rules.
//
// # Basic Usage
//
//	md := markdown.New("# Hello World\n\nThis is a **bold** statement.")
//	md.Resize(80, 24)
//	md.Draw(writer)
//
// # Scrolling
//
// The component implements component.ScrollableFloating, allowing it to be
// scrolled when content exceeds the viewport:
//
//	md.SeekDown() // scroll down one line
//	md.SeekUp()   // scroll up one line
//
// # Customization
//
// Use NewWithConfig to customize colors and styling:
//
//	cfg := markdown.DefaultConfig()
//	cfg.H1 = term.Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold}
//	md := markdown.NewWithConfig(content, cfg)
package markdown
