# Rune

Rune is a fast, GPU-rendered, keyboard-driven IDE for power users. The Unix way, finished
as a product: code, terminals, CLI tools, language intelligence, debugging, and AI agents,
all in one composable, multi-workspace environment.

Rune Agent, in `cmd/rune-agent`, is an AI coding agent shipped as an extension rather than
part of the core editor. This keeps Rune suitable for automatic programming while
still allowing uncorrupted manual programming, and it pushes the extension
system to support complex applications.

Rune has been slowly developed over the course of the last few years and we've taken great
care in making it easy to develop and maintain. We hope you enjoy hacking it as much as
you enjoy using it.

See [docs.rune.build](https://docs.rune.build) for the full documentation.

![Rune in the Romero theme](https://assets.rune.build/images/screenshots/screenshot_1_romero.webp)

## Repository layout

- `cmd/rune` — the main Rune application
- `cmd/rune-agent` — the Rune Agent extension and packages

See [AGENTS.md](AGENTS.md) for a deeper tour of the architecture.

## Makefile

```bash
make                 # build all binaries into bin/
make debug           # build with the race detector and debug-only commands enabled
make clean           # remove bin/ and target/

# individual binaries
make rune            # the editor (bin/rune)
make rune-agent      # the agent extension binary (bin/rune-agent)
make ox-api          # the API server (bin/ox-api)

# testing and code quality
make test            # run the test suite with the race detector
make test-no-race    # run the test suite without the race detector
make coverage        # generate a coverage report
make lint            # run golangci-lint
make format          # run go fmt
make generate        # regenerate generated files (protobufs, mocks, docs)

# license headers
make license         # add the license header to files that are missing one
make assert_license  # fail if any file is missing the canonical header
```

The remaining targets (`dist`, `release`, `rune-dmg*`, `*-docker-*`,
`oxprobe-*`, `*-notarize`, `*-dist*`) drive Unstable Build's internal
release, packaging, and cloud deployment pipelines and are not expected to
work outside that environment.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

To report a security issue, see [SECURITY.md](SECURITY.md).

## Sponsorship

Rune is developed by Unstable Build, LLC, a self-funded company. If you'd like to
financially support us, you can do so via GitHub Sponsors. There are no perks or
entitlements associated with sponsorship.

## License

Rune is licensed under the [GNU General Public License, version 3](LICENSE)
or, at your option, any later version.
