use crate::utils::text;

pub struct Service;

impl Service {
    pub fn new() -> Service {
        Service
    }
}

// Defined but never referenced as service::boot; resolves through the
// definitions phase.
pub fn boot() {
    let _ = text::slugify("boot");
}

pub fn run() {
    helper();
}

fn helper() {}