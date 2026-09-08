use crate::geometry;

pub struct Circle {
    radius: f64,
}

impl Circle {
    pub fn new(radius: f64) -> Circle {
        Circle { radius }
    }

    pub fn perimeter(&self) -> f64 {
        geometry::circumference(self.radius)
    }
}

pub struct Rectangle {
    width: f64,
    height: f64,
}