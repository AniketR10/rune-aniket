pub fn run() -> i32 {
    foo(1) + foo(2)
}

fn foo(a: i32) -> i32 {
    a
}

fn bar(a: i32) -> i32 {
    a + 1
}
