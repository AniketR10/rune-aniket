expect_metadata(
    id = "xsandbox_fixture",
    permissions = ["permcmd", "permnoti", "permfs", "permcfg", "permed"],
)
config({"greeting": "hello"})
fs.write("README.md", "# readme\n")

expect_rpc("config.Config/Get")
expect_rpc("workspace.Files/Read")
expect_rpc("workspace.Scheme/Watch")

h = expect_command("hello")
invoke_command(h, args = ["world"])
expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": "hello world", "level": present()},
    timeout = "5s",
)

fs.write("trigger.txt", "x\n")
expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": "saw trigger.txt"},
    timeout = "5s",
)

expect_rpc("text.Editor/SubscribeEvent")
publish_event("open", uri = "file:///tmp/opened.txt")
expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": "opened opened.txt"},
    timeout = "5s",
)

wait_idle("200ms")
assert_no_unexpected_rpcs(ignore = [
    "workspace.Scheme/*",
    "workspace.Files/*",
    "text.Editor/*",
    # The filesystem watcher may coalesce or duplicate events
    # (Create+Write), so the number of "saw trigger.txt"
    # notifications is platform dependent.
    "browser.Notifications/Notify",
])
