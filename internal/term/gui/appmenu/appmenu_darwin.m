// Copyright (C) 2017-2026 The Rune Authors
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


#import <Cocoa/Cocoa.h>

#include "_cgo_export.h"

@interface RuneMenuTarget : NSObject
- (void)runeMenuAction:(id)sender;
@end

@implementation RuneMenuTarget
- (void)runeMenuAction:(id)sender {
  runeAppMenuActivate((int)[(NSMenuItem *)sender tag]);
}
@end

// All state below is touched only from the main thread, which is the
// documented contract of runeAppMenu* and where AppKit requires menu
// mutation to happen anyway.
static RuneMenuTarget *gTarget = nil;
static NSMenu *gMainMenu = nil;
// gMenuStack is the path from the top-level menu down to the submenu
// currently being filled. Items are always appended to its last entry,
// so nested submenus push a child and pop back when finished.
static NSMutableArray<NSMenu *> *gMenuStack = nil;

static NSMenu *runeCurrentMenu(void) {
  return [gMenuStack lastObject];
}

void runeAppMenuBegin(void) {
  if (gTarget == nil) {
    gTarget = [[RuneMenuTarget alloc] init];
  }
  gMainMenu = [[NSMenu alloc] initWithTitle:@""];
  if (gMenuStack == nil) {
    gMenuStack = [[NSMutableArray alloc] init];
  }
  [gMenuStack removeAllObjects];
}

void runeAppMenuAddMenu(const char *title) {
  NSString *name = [NSString stringWithUTF8String:title];
  NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:name action:NULL keyEquivalent:@""];
  NSMenu *menu = [[NSMenu alloc] initWithTitle:name];
  [item setSubmenu:menu];
  [gMainMenu addItem:item];
  [gMenuStack removeAllObjects];
  [gMenuStack addObject:menu];
}

// runeAppMenuBeginSubmenu adds a submenu item under the current menu
// and descends into it, so subsequent items land in the child until
// runeAppMenuEndSubmenu pops back.
void runeAppMenuBeginSubmenu(const char *title) {
  NSString *name = [NSString stringWithUTF8String:title];
  NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:name action:NULL keyEquivalent:@""];
  NSMenu *menu = [[NSMenu alloc] initWithTitle:name];
  [item setSubmenu:menu];
  [runeCurrentMenu() addItem:item];
  [gMenuStack addObject:menu];
}

void runeAppMenuEndSubmenu(void) {
  if ([gMenuStack count] > 1) {
    [gMenuStack removeLastObject];
  }
}

void runeAppMenuAddSeparator(void) {
  [runeCurrentMenu() addItem:[NSMenuItem separatorItem]];
}

void runeAppMenuAddItem(const char *title, const char *selector,
                        const char *keyEquiv, unsigned long modifiers, int tag,
                        int disabled, int checked) {
  NSMenuItem *item =
      [[NSMenuItem alloc] initWithTitle:[NSString stringWithUTF8String:title]
                                 action:NULL
                          keyEquivalent:[NSString stringWithUTF8String:keyEquiv]];
  [item setKeyEquivalentModifierMask:(NSEventModifierFlags)modifiers];
  [item setState:(checked ? NSControlStateValueOn : NSControlStateValueOff)];

  if (disabled) {
    // A disabled item has no action, so AppKit greys it out.
    [item setEnabled:NO];
  } else if (selector[0] != '\0') {
    // Target nil sends the action down the responder chain, which is
    // what standard AppKit actions expect.
    [item setAction:NSSelectorFromString([NSString stringWithUTF8String:selector])];
  } else {
    [item setTag:tag];
    [item setTarget:gTarget];
    [item setAction:@selector(runeMenuAction:)];
  }

  [runeCurrentMenu() addItem:item];
}

void runeAppMenuCommit(void) {
  [NSApp setMainMenu:gMainMenu];
  [gMenuStack removeAllObjects];
}
