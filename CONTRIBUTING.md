# Contributing to mpckit

Thanks for helping. Bug reports, device test results and pull requests are all useful.

## Before you start

- For anything bigger than a small fix, open an issue first so we can agree on the approach.
- The current scope is in [docs/build-plan.md](docs/build-plan.md), section 1. Features listed there as "not in 1.0" are welcome as proposals, but may wait for a later release.

## Setting up

See "Building from source" in the [README](README.md). All Go, Wails and npm commands run from `src/`.

## The one rule that matters most

The MPC Sample only accepts project files that stay very close to what it writes itself. mpckit therefore:

- changes only the fields it knows about and writes everything else back exactly as it was read (key order and number text included);
- has golden-file tests in `src/testdata/` that compare its output with files known to load on the device.

If your change affects how `.xpj` files or exports are written:

- add or update a test;
- never edit the files in `src/testdata/device/` or `src/testdata/accepted/`, they are device-verified fixtures;
- say in the pull request whether you loaded the result on an MPC Sample, and what happened.

The file format notes are in [docs/xpj-format-and-scope.md](docs/xpj-format-and-scope.md).

## Code layout

- `src/internal/xpj`, `wav`, `project`, `export`: file handling. These must not import Wails, so they can be tested with plain `go test`.
- `src/internal/app` and `src/main.go`: the Wails services and window.
- `src/frontend`: the Svelte + TypeScript UI.

## Pull requests

- Keep each pull request to one change.
- Run `go vet ./...` and `go test ./...` from `src/` before pushing. CI builds and tests on Linux, Windows and macOS.
- Fill in the pull request template.
- Don't commit audio you don't have the right to share. Factory samples copied from the device belong to Akai/inMusic.

## License

By contributing, you agree that your contribution is licensed under the [MIT License](LICENSE) like the rest of the project.
