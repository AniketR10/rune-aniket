# The "api" command exercises every reachable service method, but never
# opens a floating window (Floating needs a live browser to allocate
# one, so the sandbox can't complete the round-trip headlessly). This
# spec expects a WindowManager/Floating RPC that never arrives, so the
# sandbox must fail loudly rather than pass.
expect_metadata(id = "xsandbox_sample")
config({"prefix": "demo"})
fs.write("VERSION", "9.9.9\n")

a = expect_command("api")
invoke_command(a, timeout = "20s")

expect_rpc("browser.WindowManager/Floating", timeout = "1s")
