//! Fixture for the multi-edit code-action path. Its @import decls are
//! interleaved with plain decls and separated by a doc comment, which
//! makes zls emit "organize @import" as one insert plus several
//! whole-line deletions that share a start position with it. That edit
//! shape is what a naive front-to-back application corrupts.

const std = @import("std");
const testing = std.testing;
const lib = @import("lib.zig");
const CAllocator = lib.add;
const io_c = @import("main.zig");
const Result = @import("lib.zig").add;
const snapshot_core = @import("main.zig");
const apc = @import("lib.zig");

/// Doc comment that must stay attached to the decl below.
const default_max_continuation_bytes = 3;
const terminal_c = @import("main.zig");

/// C: doc comment for the next decl.
pub fn use() i32 {
    _ = testing;
    _ = CAllocator;
    _ = io_c;
    _ = Result;
    _ = snapshot_core;
    _ = apc;
    _ = terminal_c;
    return default_max_continuation_bytes;
}
