# mpckit: notes for Claude

mpckit is a desktop editor for Akai MPC Sample projects (`.xpj` files plus their `_[ProjectData]` sample folder). Go, Wails v3, Svelte + TypeScript + Vite.

## Read first

- `docs/xpj-format-and-scope.md`: the file format, verified edit recipes and the agreed 1.0 scope.
- `docs/build-plan.md`: architecture, service API, milestones, device checkpoints.

## Rules

- `internal/xpj`, `internal/wav`, `internal/project` and `internal/export` must not import Wails. Only `internal/app` and `main.go` touch Wails.
- Round trip: unchanged content must re-write byte-identical (key order and number text kept). Never use `map[string]any` + `json.Marshal` for project data.
- Never modify fixtures in `testdata/device/` or `testdata/accepted/`; they are files verified on the hardware.
- Stay inside the 1.0 scope unless asked. 2.0 items are listed in the format doc.

## Repository layout (GitHub)

The project moved from a private Gogs server to https://github.com/bajdev/MPCkit at beta, with a fresh history.

- `src/`: the whole Wails app (`go.mod`, `main.go`, `internal/`, `frontend/`, `build/`, `Taskfile.yml`, `testdata/`). Run every Go, Wails and npm command from `src/`.
- `docs/`: format notes and the build plan. Paths in these docs were written before the move and are relative to `src/` (for example `internal/xpj` means `src/internal/xpj`).
- `.github/`: CI and contributor templates.

Commands, from `src/`:

```sh
go vet ./... && go test ./...
wails3 dev
wails3 build
```

## Working on GitHub

- `main` is protected: every change goes through a pull request, and the three `Build (...)` checks must pass.
- Branch names: short and descriptive, e.g. `fix/export-windows-paths`, `feat/appimage`.
- CI (`.github/workflows/build.yml`) builds Linux, Windows and macOS on every pull request and runs `go vet` and `go test` on each. A pushed `v*` tag also creates a draft release with all three builds. Tags with a hyphen (`v1.0.0-beta.1`) and 0.x tags (`v0.5.5`) become pre-releases.
- Release: on an up-to-date `main`, tag `vX.Y.Z` (0.x while in beta), push the tag, check the draft release on GitHub, then publish it.
- The app version comes from the tag. CI passes `VERSION=<tag without v>` to the Wails tasks; the Taskfile must pass it into the build (for example `-ldflags "-X main.version={{.VERSION}}"`) so the About window shows it.
- Reviewing outside pull requests: check that changes to file output come with tests; read code before running it locally; look closely at any change under `.github/` or `src/build/`, since those run in CI.
- The private Gogs repo is archived. Don't reference it in new code or docs.
