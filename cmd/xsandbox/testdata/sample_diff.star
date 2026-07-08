# Deliberately diverges from what testdata/sampleext does: it invokes
# "greet ada" but expects the greeting to name "bob". The sandbox must
# catch the mismatch and report the observed notification.
expect_metadata(id = "xsandbox_sample")
config({"prefix": "demo"})
fs.write("VERSION", "9.9.9\n")

g = expect_command("greet")
invoke_command(g, args = ["ada"])
expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": "demo: hi bob"},
    timeout = "1s",
)
