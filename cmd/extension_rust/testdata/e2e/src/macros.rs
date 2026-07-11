macro_rules! answer {
    () => {
        42
    };
}

pub fn value() -> i32 {
    answer!()
}
