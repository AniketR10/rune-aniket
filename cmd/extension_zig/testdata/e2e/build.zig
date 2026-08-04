const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const exe = b.addExecutable(.{
        .name = "rune_zig_ext_e2e",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });
    b.installArtifact(exe);

    // The "check" step compiles without installing binaries. zls
    // auto-enables build-on-save when a build.zig declares it.
    const exe_check = b.addExecutable(.{
        .name = "rune_zig_ext_e2e_check",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });
    const check = b.step("check", "Typecheck the project without installing");
    check.dependOn(&exe_check.step);
}
