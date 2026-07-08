# Drives testdata/sampleext's "wm" command, which installs tui.Handlers
# through the window manager. Asserts the sandbox records the
# handler-carrying streams (Split, Tab, Bar) and their notifications.
expect_metadata(id = "xsandbox_sample")
config({"prefix": "demo"})
fs.write("VERSION", "9.9.9\n")

c = expect_command("wm")
invoke_command(c, timeout = "15s")

expect_rpc("browser.WindowManager/Split")
expect_rpc("browser.WindowManager/Tab")
expect_rpc("browser.WindowManager/Bar")
expect_rpc("browser.Notifications/Notify", where = {"msg": "demo: wm split ok"})
expect_rpc("browser.Notifications/Notify", where = {"msg": "demo: wm tab ok"})
expect_rpc("browser.Notifications/Notify", where = {"msg": "demo: wm bar ok"})
