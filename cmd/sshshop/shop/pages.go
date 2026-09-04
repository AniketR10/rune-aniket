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

package shop

// page is a named entry in the storefront. Every page carries a fixed
// markdown body; in later iterations a page will be able to produce
// arbitrary tui.Handlers so we can plug in a checkout form, a product
// carousel, etc. The first entry (home) is opened by default in the
// initial left tile.
type page struct {
	name    string
	summary string
	body    string
}

// pages returns the ordered list of storefront pages. They are all
// opened as tabs at session start. The first entry is the home page
// and is the one shown on the initial focused window.
func pages() []page {
	return []page{
		{
			name:    "home",
			summary: "Storefront landing page",
			body:    homeMarkdown,
		},
		{
			name:    "about",
			summary: "What this storefront is",
			body:    aboutMarkdown,
		},
		{
			name:    "product",
			summary: "Browse the catalog",
			body:    productMarkdown,
		},
		{
			name:    "pricing",
			summary: "Plans and billing",
			body:    pricingMarkdown,
		},
		{
			name:    "account",
			summary: "Your account and order history",
			body:    accountMarkdown,
		},
	}
}

const homeMarkdown = `# Welcome to sshshop

A terminal-first storefront served entirely over SSH.

## Instructions

- Press ` + "`<ctrl-p>`" + ` to open the command palette. From the palette you
  can navigate between pages, split/close windows, focus tabs, and
  more. Type ` + "`help`" + ` to list every available command.
- Use the **shell** on the bottom-right window to run storefront
  commands directly:
    - ` + "`sign-up`" + ` — create an account.
    - ` + "`sign-in`" + ` — log into an existing account.
    - ` + "`download`" + ` — download the sshshop CLI.
    - ` + "`help`" + ` — list every shell command.
- Tabs live at the top of the browser. Switch between them with
  ` + "`<ctrl-l>`" + ` / ` + "`<ctrl-h>`" + ` (next / previous) or by clicking.
- The **mouse** works too: click a tab to focus it, click inside a
  window to focus that window, scroll to scroll.
- Press ` + "`<ctrl-c>`" + ` to disconnect from the storefront.
`

const aboutMarkdown = `# About

**sshshop** is an SSH-accessible storefront, built as a playground for
TUI-first commerce. The entire experience lives inside your terminal —
no browser, no web stack.

## How this works

- You ` + "`ssh`" + ` into the shop.
- The server terminates the SSH channel in-process and renders a
  rune-go-sdk TUI directly against the session byte stream.
- No real user accounts, no real shell, no sandboxing needed.

Close this page with ` + "`q`" + ` or ` + "`Esc`" + ` to return to the command prompt.
`

const productMarkdown = `# Product

Sample catalog entry. Replace this with real products.

## Coffee — Single Origin

- **Origin**: Ethiopia, Yirgacheffe
- **Roast**: Light
- **Notes**: bergamot, jasmine, stone fruit
- **Price**: $22 / 340g

## Coffee — Espresso Blend

- **Origin**: Brazil + Colombia
- **Roast**: Medium-dark
- **Notes**: cocoa, caramel, hazelnut
- **Price**: $18 / 340g

_Pick an item with arrow keys, Enter to add to cart (placeholder)._
`

const pricingMarkdown = `# Pricing

Placeholder plans.

| Plan    | Price/mo | Shipping    | Notes                        |
|---------|----------|-------------|------------------------------|
| Taster  | $0       | Pay per bag | Single bag, no subscription  |
| Regular | $20      | Free        | One bag / month              |
| Curious | $36      | Free        | Two bags / month, rotating   |

Real pricing, payment capture and subscription management will land in
a subsequent iteration. Payments are processed via Stripe so that card
data never touches this server.
`

const accountMarkdown = `# Account

You are browsing as an anonymous guest.

To link orders to an email you will log in **inside the TUI**, via a
magic-link code sent to your inbox. SSH keys are not used for identity
on purpose — the shop should work with ` + "`ssh sshshop.example`" + ` and
nothing else.

## Order history

_(No orders yet.)_
`
