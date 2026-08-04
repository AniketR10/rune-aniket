pub fn area(radius: f64) f64 {
    return 3.14159 * radius * radius;
}

// Defined but never referenced as geometry.perimeter; resolves only
// through the definitions phase.
pub fn perimeter(radius: f64) f64 {
    return 2.0 * 3.14159 * radius;
}

pub const Shape = struct {
    size: f64,

    pub fn init(size: f64) Shape {
        return .{ .size = size };
    }
};

const Internal = struct {};

fn make_internal() Internal {
    return .{};
}
