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

#include <string.h>

#include "_cgo_export.h"

// runeOpenPanelShow must run on the main thread. The panel is shown
// with a completion handler rather than runModal so the caller's event
// loop keeps running; the completion block fires on the main thread
// during normal event dispatch and reports the selection to Go as a
// NUL-joined path list. An empty payload signals cancellation.
//
// glfw does not regain key-window status once the panel dismisses, so
// keystrokes would be dropped (and beep) until the user clicks the
// window. The completion handler reactivates the app and makes the
// host window key and front to hand keyboard focus back. The host is
// resolved defensively: when a menu item triggers the panel the app
// has no key window (the menu bar is tracking), so mainWindow and the
// window list are consulted as fallbacks.
void runeOpenPanelShow(int tag, const char *title, const char *message,
                       const char *prompt, int directories, int multiple) {
  NSOpenPanel *panel = [NSOpenPanel openPanel];
  if (title[0] != '\0') {
    [panel setTitle:[NSString stringWithUTF8String:title]];
  }
  if (message[0] != '\0') {
    [panel setMessage:[NSString stringWithUTF8String:message]];
  }
  if (prompt[0] != '\0') {
    [panel setPrompt:[NSString stringWithUTF8String:prompt]];
  }
  [panel setCanChooseFiles:(directories ? NO : YES)];
  [panel setCanChooseDirectories:(directories ? YES : NO)];
  [panel setAllowsMultipleSelection:(multiple ? YES : NO)];

  NSWindow *host = [NSApp keyWindow];
  if (host == nil) {
    host = [NSApp mainWindow];
  }
  if (host == nil) {
    for (NSWindow *w in [NSApp windows]) {
      if ([w canBecomeKeyWindow]) {
        host = w;
        break;
      }
    }
  }

  void (^handler)(NSModalResponse) = ^(NSModalResponse response) {
    // The result is collected synchronously: the completion handler
    // owns the panel only for its own duration, and this file is
    // compiled under manual retain/release, so the panel must not be
    // touched from the deferred refocus block below.
    if (response == NSModalResponseOK) {
      NSMutableData *joined = [NSMutableData data];
      for (NSURL *url in [panel URLs]) {
        const char *path = [url fileSystemRepresentation];
        if (path == NULL || path[0] == '\0') {
          continue;
        }
        if ([joined length] > 0) {
          [joined appendBytes:"\0" length:1];
        }
        [joined appendBytes:path length:strlen(path)];
      }
      runeOpenPanelDone(tag, (char *)[joined bytes], (int)[joined length]);
    } else {
      runeOpenPanelDone(tag, NULL, 0);
    }

    // Hand keyboard focus back to the app window; the panel taking key
    // status leaves glfw unfocused until the user clicks otherwise.
    // Deferred to the next runloop turn so it runs after AppKit has
    // finished dismissing the panel and is not overridden by the
    // teardown's own focus handling. host is retained across the async
    // hop because captured objects are not retained under manual
    // retain/release.
    NSWindow *refocus = [host retain];
    dispatch_async(dispatch_get_main_queue(), ^{
      [NSApp activateIgnoringOtherApps:YES];
      if (refocus != nil) {
        [refocus makeKeyAndOrderFront:nil];
      }
      [refocus release];
    });
  };

  if (host != nil) {
    [panel beginSheetModalForWindow:host completionHandler:handler];
  } else {
    [panel beginWithCompletionHandler:handler];
  }
}
