mod geometry;
mod shapes;

use crate::geometry::area;
use crate::shapes::Circle;

fn main() {
    let r = 3.0;
    let a = geometry::area(r);
    let circle = shapes::Circle::new(r);
    let p = circle.perimeter();
    println!("{} {} {}", a, area(r), p);
    let _ = Circle::new(1.0);
}