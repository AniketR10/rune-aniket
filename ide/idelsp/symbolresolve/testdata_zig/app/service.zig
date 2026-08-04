const text = @import("../utils/text.zig");

pub const Service = struct {
    pub fn init() Service {
        return .{};
    }
};

// Defined but never referenced as service.boot; resolves through the
// definitions phase.
pub fn boot() void {
    _ = text.slugify("boot");
}

pub fn run() void {
    helper();
}

fn helper() void {}
