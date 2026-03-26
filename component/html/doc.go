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

// Package html provides a TUI component that fetches HTML content from
// a URL, converts it to markdown using html-to-markdown, and renders it
// using the markdown component.
//
// While loading, the component displays a progress animation. Once loaded,
// it displays the converted markdown content. The component is immutable
// after construction: each instance represents a single URL fetch.
package html
