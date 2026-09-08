pub fn area(radius: f64) -> f64 {
    std::f64::consts::PI * radius * radius
}

// Defined but never referenced as geometry::perimeter; resolves only
// through the definitions phase.
pub fn perimeter(radius: f64) -> f64 {
    2.0 * std::f64::consts::PI * radius
}

pub struct Shape {
    size: f64,
}

impl Shape {
    pub fn new(size: f64) -> Shape {
        Shape { size }
    }
}

struct Internal;

fn make_internal() -> Internal {
    Internal
}