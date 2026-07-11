pub trait Marker {}

pub struct Marked;

impl Marker for Marked {}

pub fn eval_here() {
    let _x = 1;
}
