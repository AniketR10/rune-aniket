expect_metadata(id = "crash")

expect_rpc("config.Config/Get", timeout = "2s")
