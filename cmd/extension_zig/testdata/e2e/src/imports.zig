// The @import declarations are deliberately out of order so zls offers
// its "organize @import" source action.
const lib = @import("lib.zig");
const std = @import("std");

pub fn sum(a: i32, b: i32) i32 {
    std.debug.assert(a >= 0);
    return lib.add(a, b);
}
