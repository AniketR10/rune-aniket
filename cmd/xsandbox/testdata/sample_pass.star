# Mirrors exactly what testdata/sampleext does, so the sandbox run
# passes: config read, VERSION read + notify, command registrations,
# command invocations, and editor open-event subscription + notify.
expect_metadata(
    id = "xsandbox_sample",
    permissions = [
        "permcmd", "permnoti", "permfs", "permcfg", "permed",
        "permexec", "permpty", "permstore", "permwm", "permopen",
        "permint", "permsyntax", "permlsp", "permdap", "permllm",
    ],
)
config({"prefix": "demo"})
fs.write("VERSION", "9.9.9\n")

expect_rpc("config.Config/Get")
expect_rpc("workspace.Files/Read")
expect_rpc("text.Editor/SubscribeCommand")
expect_rpc("text.Editor/SubscribeEvent")
expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": "demo: version 9.9.9"},
    timeout = "5s",
)

g = expect_command("greet")
invoke_command(g, args = ["ada"])
expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": "demo: hi ada", "level": present()},
    timeout = "5s",
)

c = expect_command("count")
invoke_command(c, args = ["a", "b", "c"])
expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": regex("^demo: n=3$")},
    timeout = "5s",
)

publish_event("open", uri = "file:///tmp/foo.go")
expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": "demo: opened foo.go"},
    timeout = "5s",
)

wait_idle("200ms")
