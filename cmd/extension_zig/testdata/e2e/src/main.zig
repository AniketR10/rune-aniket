const std = @import("std");
const lib = @import("lib.zig");

pub fn main() void {
    const result = lib.add(2, 3);
    std.debug.print("{d}\n", .{result});
}
