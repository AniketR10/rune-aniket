# llamacpp testdata

This directory is used by the opt-in black-box service test suite in
`service_test.go` (the `TestService_Testdata_*` tests). Real GGUF model files
and their companion mmproj blobs are downloaded into `testdata/cache/` on
demand and cached between runs.

The downloaded files are gitignored; see `.gitignore` at the repo root.

## Opt-in

```bash
RUNE_LLAMACPP_TESTDATA=1 \
    go test -v -race -run 'TestService_Testdata' \
    ./cmd/rune-agent/llm/llamacpp/...
```

Additional environment variables:

- `RUNE_LLAMACPP_TESTDATA_OFFLINE=1` — never hit the network; skip if the
  model is not already cached locally.
- `RUNE_LLAMACPP_TESTDATA_DIR=/path/to/cache` — override the cache directory
  (defaults to `testdata/cache/`).

## Default model

`ggml-org/gemma-4-E2B-it-GGUF:Q8_0` (~4.6 GiB weight) plus the matching Q8_0
mmproj blob (~0.8 GiB). Gemma-4 E2B is small enough to load on a laptop while
still carrying useful tool-calling and vision capabilities. The choice is
fixed because the tests make model-specific assertions against the chat
template and tool-calling behaviour.
