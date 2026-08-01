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
static NSMenu *gCurrentMenu = nil;

void runeAppMenuBegin(void) {
  if (gTarget == nil) {
    gTarget = [[RuneMenuTarget alloc] init];
  }
  gMainMenu = [[NSMenu alloc] initWithTitle:@""];
  gCurrentMenu = nil;
}

void runeAppMenuAddMenu(const char *title) {
  NSString *name = [NSString stringWithUTF8String:title];
  NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:name action:NULL keyEquivalent:@""];
  gCurrentMenu = [[NSMenu alloc] initWithTitle:name];
  [item setSubmenu:gCurrentMenu];
  [gMainMenu addItem:item];
}

void runeAppMenuAddSeparator(void) {
  [gCurrentMenu addItem:[NSMenuItem separatorItem]];
}

void runeAppMenuAddItem(const char *title, const char *selector,
                        const char *keyEquiv, unsigned long modifiers, int tag) {
  NSMenuItem *item =
      [[NSMenuItem alloc] initWithTitle:[NSString stringWithUTF8String:title]
                                 action:NULL
                          keyEquivalent:[NSString stringWithUTF8String:keyEquiv]];
  [item setKeyEquivalentModifierMask:(NSEventModifierFlags)modifiers];

  if (selector[0] != '\0') {
    // Target nil sends the action down the responder chain, which is
    // what standard AppKit actions expect.
    [item setAction:NSSelectorFromString([NSString stringWithUTF8String:selector])];
  } else {
    [item setTag:tag];
    [item setTarget:gTarget];
    [item setAction:@selector(runeMenuAction:)];
  }

  [gCurrentMenu addItem:item];
}

void runeAppMenuCommit(void) {
  [NSApp setMainMenu:gMainMenu];
  gCurrentMenu = nil;
}
