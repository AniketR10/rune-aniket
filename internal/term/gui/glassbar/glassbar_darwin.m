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

// NSBezelStyleGlass ships in macOS 26. Rune's deployment target is
// older, so it is referenced by raw value under @available and the bar
// falls back to a vibrancy-backed circular button on earlier releases.
enum { kRuneBezelStyleGlass = 16 };

@interface RuneGlassButton : NSButton
@property(copy) NSString *runeID;
@end

@implementation RuneGlassButton
@end

@interface RuneGlassTarget : NSObject
- (void)runeGlassAction:(id)sender;
@end

@implementation RuneGlassTarget
- (void)runeGlassAction:(id)sender {
  RuneGlassButton *button = (RuneGlassButton *)sender;
  runeGlassBarActivate((char *)[button.runeID UTF8String]);
}
@end

// All state below is touched only from the main thread, which is the
// documented contract of runeGlassBar* and where AppKit requires view
// hierarchy mutation to happen anyway.
static RuneGlassTarget *gTarget = nil;
static NSView *gContainer = nil;
static NSStackView *gStack = nil;
static NSMutableArray<NSLayoutConstraint *> *gSizeConstraints = nil;

static NSWindow *runeGlassWindow(void) {
  NSWindow *window = [NSApp mainWindow];
  if (window != nil) {
    return window;
  }
  return [[NSApp windows] firstObject];
}

void runeGlassBarBegin(void) {
  if (gTarget == nil) {
    gTarget = [[RuneGlassTarget alloc] init];
  }
  [gContainer removeFromSuperview];
  gContainer = nil;
  gSizeConstraints = [[NSMutableArray alloc] init];

  gStack = [[NSStackView alloc] initWithFrame:NSZeroRect];
  gStack.orientation = NSUserInterfaceLayoutOrientationVertical;
  gStack.alignment = NSLayoutAttributeCenterX;
  gStack.distribution = NSStackViewDistributionFill;
  gStack.translatesAutoresizingMaskIntoConstraints = NO;
}

void runeGlassBarAddButton(const char *id, const char *symbol,
                           const char *tooltip) {
  if (gStack == nil) {
    return;
  }
  NSString *symbolName = [NSString stringWithUTF8String:symbol];
  NSString *help = [NSString stringWithUTF8String:tooltip];

  RuneGlassButton *button = [[RuneGlassButton alloc] initWithFrame:NSZeroRect];
  button.runeID = [NSString stringWithUTF8String:id];
  button.target = gTarget;
  button.action = @selector(runeGlassAction:);
  button.title = @"";
  button.toolTip = help;
  button.bordered = YES;
  button.imageScaling = NSImageScaleProportionallyDown;
  button.image = [NSImage imageWithSystemSymbolName:symbolName
                           accessibilityDescription:help];
  button.translatesAutoresizingMaskIntoConstraints = NO;
  if (@available(macOS 26.0, *)) {
    button.bezelStyle = (NSBezelStyle)kRuneBezelStyleGlass;
  } else {
    button.bezelStyle = NSBezelStyleCircular;
  }

  NSLayoutConstraint *width =
      [button.widthAnchor constraintEqualToConstant:0];
  NSLayoutConstraint *height =
      [button.heightAnchor constraintEqualToConstant:0];
  [gSizeConstraints addObject:width];
  [gSizeConstraints addObject:height];
  [NSLayoutConstraint activateConstraints:@[ width, height ]];
  [gStack addArrangedSubview:button];
}

void runeGlassBarCommit(void) {
  NSWindow *window = runeGlassWindow();
  if (window == nil || gStack == nil || gStack.arrangedSubviews.count == 0) {
    gStack = nil;
    return;
  }
  NSView *content = [window contentView];
  if (content == nil) {
    gStack = nil;
    return;
  }

  if (@available(macOS 26.0, *)) {
    // The glass bezel already carries the material; a backdrop would
    // only stack a second one behind every button.
    gContainer = [[NSView alloc] initWithFrame:NSZeroRect];
  } else {
    NSVisualEffectView *backdrop =
        [[NSVisualEffectView alloc] initWithFrame:NSZeroRect];
    backdrop.material = NSVisualEffectMaterialHUDWindow;
    backdrop.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    backdrop.state = NSVisualEffectStateActive;
    backdrop.wantsLayer = YES;
    gContainer = backdrop;
  }
  gContainer.autoresizingMask = NSViewMinXMargin | NSViewMinYMargin;

  [gContainer addSubview:gStack];
  [content addSubview:gContainer positioned:NSWindowAbove relativeTo:nil];
}

void runeGlassBarSetFrame(double x, double y, double width, double padding) {
  if (gContainer == nil || gStack == nil) {
    return;
  }
  NSView *content = gContainer.superview;
  NSUInteger count = gStack.arrangedSubviews.count;
  double buttonSize = width - 2 * padding;
  if (content == nil || count == 0 || buttonSize <= 0) {
    return;
  }

  gStack.spacing = padding;
  for (NSLayoutConstraint *constraint in gSizeConstraints) {
    constraint.constant = buttonSize;
  }

  double height = count * buttonSize + (count - 1) * padding + 2 * padding;
  if ([gContainer isKindOfClass:[NSVisualEffectView class]]) {
    gContainer.layer.cornerRadius = width / 2;
  }
  // The content view is not flipped, so the caller's top-left origin is
  // measured down from the top of the window.
  gContainer.frame =
      NSMakeRect(x, content.bounds.size.height - y - height, width, height);
  gStack.frame = NSMakeRect(padding, padding, buttonSize, height - 2 * padding);
}
