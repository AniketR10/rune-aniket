const std = @import("std");

pub const Greeter = struct {
    name: []const u8,

    pub fn init(name: []const u8) Greeter {
        return .{ .name = name };
    }

    pub fn greet(self: Greeter) []const u8 {
        return self.name;
    }
};

pub fn add(a: i32, b: i32) i32 {
    return a + b;
}

pub fn main() void {
    const g = Greeter.init("World");
    std.debug.print("Hello, {s}!\n", .{g.greet()});
    const result = add(1, 2);
    std.debug.print("{d}\n", .{result});
}
