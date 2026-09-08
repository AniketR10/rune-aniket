pub const PI: f64 = 3.141592653589793;

pub fn area(radius: f64) -> f64 {
    PI * radius * radius
}

// Defined but never referenced as geometry::circumference, so it resolves
// only through the definitions phase.
pub fn circumference(radius: f64) -> f64 {
    2.0 * PI * radius
}

pub struct Point {
    pub x: f64,
    pub y: f64,
}