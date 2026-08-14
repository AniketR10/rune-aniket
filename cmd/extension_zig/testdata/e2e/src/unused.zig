// Deliberately unclean source that drives zls's quickfixes: the unused
// const yields "discard value" and the never-mutated var yields "use
// 'const'", which together also populate the "apply fixall" batch. It is
// intentionally not reachable from build.zig so `zig build check` stays
// green for the build-on-save test.
pub fn Compute_Sum(a: i32, b: i32) i32 {
    var total: i32 = a + b;
    const leftover = a;
    return total;
}
