expect_metadata(id = "xsandbox_fixture")
config({"greeting": "hello"})
fs.write("README.md", "# readme\n")

expect_rpc(
    "browser.Notifications/Notify",
    where = {"msg": "never happens"},
    timeout = "500ms",
)
