pub const Response = struct {};

pub fn get(url: []const u8) Response {
    _ = url;
    return .{};
}

pub fn post(url: []const u8) Response {
    _ = url;
    return .{};
}
