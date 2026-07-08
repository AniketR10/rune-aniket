expect_metadata(id = "xsandbox_fixture", permissions = ["permcmd"])
config({"greeting": "hello"})
fs.write("README.md", "# readme\n")

expect_rpc("config.Config/Get", timeout = "2s")
