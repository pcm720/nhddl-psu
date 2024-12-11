# NHDDL PSU Generator

This repository implements a [WebAssembly-based UI](cmd/nhddl-psu) for generating PSU files from NHDDL releases.

It also includes a small [psubuilder](cmd/psubuilder) utility that can be used to generate PSU file from local files and directories or GitHub releases.

### WebAssembly UI

To build `nhddl-psu`, you'll need TinyGo (at least 0.34.0), Go (at least 1.23.4).
Then you'll need to run `make` or `make nhddl-psu`.
The compiled WASM binary and support files will be placed in the `out` directory.

Makefile environment variables (injected into the binary at build time):
- `REPO` — target repository (required)
- `CORS_PROXY` — CORS proxy URL (optional, e.g. `https://cors.example.com/`)

Note that the UI will not be able to download release assets due to some GitHub endpoints not having CORS policies. To work around this, a CORS proxy is needed.  

### `psubuilder`

To build `psubuilder`, all you need is to install Go (at least 1.23.4) and run `make psubuilder`.  
The compiled binary will be placed in the `out` directory.