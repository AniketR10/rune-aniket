mod geometry;
mod requests;

use crate::app::service;
use crate::utils::text;

fn main() {
    // Qualified references resolve to this call site via the
    // reference phase.
    let a = geometry::area(3.0);
    let shape = geometry::Shape::new(a);

    // Dependency-style module reference.
    let resp = requests::get("https://example.com");

    let svc = service::Service::new();
    let slug = text::slugify("Hello World");
    let _ = (shape, resp, svc, slug);
}