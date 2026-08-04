const geometry = @import("geometry.zig");
const requests = @import("requests.zig");
const service = @import("app/service.zig");
const text = @import("utils/text.zig");

pub fn main() void {
    // Qualified references resolve to this call site via the
    // reference phase.
    const a = geometry.area(3.0);
    const shape = geometry.Shape.init(a);

    // Dependency-style module reference.
    const resp = requests.get("https://example.com");

    const svc = service.Service.init();
    const slug = text.slugify("Hello World");
    _ = shape;
    _ = resp;
    _ = svc;
    _ = slug;
}
