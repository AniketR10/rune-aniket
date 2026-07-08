# xsandbox spec for the Color Palette extension.
#
# Run with:
#   xsandbox --spec cmd/extension_color_palette/spec.star -- ./bin/extension_color_palette
#
# The extension registers a single "colorpalette" command; invoking it
# opens a color-grid panel by splitting the focused window to the right.
# A second invocation while the panel is open is a no-op, so only one
# Split is ever issued.

expect_metadata(
    id = "color_palette",
    permissions = ["permcmd", "permed", "permwm"],
)

# The command is registered during ExtendWorkspace.
c = expect_command("colorpalette")
expect_rpc("text.Editor/SubscribeCommand")

# First invocation opens the palette by splitting the focused window.
invoke_command(c, timeout = "15s")
expect_rpc("browser.WindowManager/Split")

# The palette window is now open; a second invocation is a no-op and
# must NOT issue another Split. Consume the first Split above, then
# assert no further window-manager calls arrive.
invoke_command(c, timeout = "15s")
wait_idle("1s")
assert_no_unexpected_rpcs(ignore = ["text.Editor/*", "browser.WindowManager/Focus"])

# Render the installed palette handler and assert its exact content.
# The grid draws the 256-color palette hex codes; at this width the
# top row shows the #00 swatch. Sending <c-d> toggles dim mode, which
# prefixes every color name/code with "D".
w = expect_window(method = "browser.WindowManager/Split", timeout = "5s")

bright = render(w, width = 120, height = 40)
if "#00" not in bright:
    fail("expected palette hex #00 in render, got:\n" + bright)
if "D#0" in bright:
    fail("did not expect dim-prefixed codes before <c-d>, got:\n" + bright)

send_key(w, "<c-d>")
dimmed = render(w, width = 120, height = 40)
if "D#0" not in dimmed:
    fail("expected dim-prefixed codes after <c-d>, got:\n" + dimmed)
