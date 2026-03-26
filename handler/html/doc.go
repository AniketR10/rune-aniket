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

// Package html provides a TUI handler that fetches and renders HTML pages
// as markdown with less-like keyboard navigation, mouse text selection,
// link handling, and fetch cancellation.
//
// While loading, the handler accepts Ctrl+C to cancel and q/Esc to
// exit. Once the content is loaded, it provides full interactive
// rendering with scrolling, selection, and link clicks. Clicking an
// http/https link triggers navigation to the new URL. Previously
// visited pages are cached so that navigating back does not require a
// new fetch.
package html
