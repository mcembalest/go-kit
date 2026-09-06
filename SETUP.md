# Set up an app

Start with an app that has root `go.mod` and `main.go`. Copy these files into its
repository, replacing `myapp` with its executable name:

| Source in go-kit | Destination in the app |
| --- | --- |
| `templates/app/Makefile` | `Makefile` |
| `templates/app/ci.yaml` | `.github/workflows/ci.yaml` |
| `templates/app/goreleaser.yaml` | `.goreleaser.yaml` |
| `templates/app/DEPENDENCIES.md` | `DEPENDENCIES.md` |
| `templates/common/version.go` | `version.go` |

Merge with existing configuration; keep the app's existing checks. Fill out the
dependency notes and link them from its README. Add `/bin/` and `/dist/` to its
`.gitignore`. No reference to the go-kit checkout is needed afterward.

Add an early return in `main`, before dependency setup or opening anything:

```go
if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
    fmt.Println(buildVersion("myapp"))
    return
}
```

Include `fmt` and `os` in the entry point's imports. The snippet reads Go's
embedded module/VCS metadata; GoReleaser supplies archive version and commit.
Unavailable metadata is reported as unknown.

## Keep the commands app-owned

| Command | Meaning |
| --- | --- |
| `make run` | Run the app locally; pass arguments with `ARGS='...'` |
| `make check` | Go race tests and vet, plus the app's own checks |
| `make build` | Build the current app into `bin/` |
| `make release-check` | Checks and local snapshot archives; no publishing |

The starting workflow tests on macOS and Linux and packages amd64/arm64 builds.
Narrow those targets to systems the app actually supports. Cross-compilation is
not a runtime test. The default configuration assumes no CGO; adapt it if the app
needs native libraries. Use GoReleaser OSS v2.18.0 locally to match the workflow.

For browser assets, add the app's npm build/tests to the appropriate recipes and
install Node/dependencies in CI. Commit the built assets needed by `go:embed` so
`go install` requires no Node build step. Have CI rebuild and check those assets
against Git; do not automatically accept changed visual baselines. Keep Python
example/tool environments separate and use uv where appropriate.

For the default Go + TypeScript shape, keep the same public Make targets:

- `run` and `build`: rebuild the UI before starting/building Go.
- `check`: run TypeScript checks and UI tests, Go checks, then fail if rebuilding
  changed the committed embedded assets or created untracked assets there.
- CI: install the app's Node version and locked npm dependencies before checks.
  The release job may build from verified committed assets without Node.

Keep frontend paths, npm scripts, and browser test platforms in the app's own
configuration. There is no mandatory frontend directory or additional shared
command. For another language, follow the same rule: its native tools join the
existing targets only where needed, and runtime dependencies are explicit.

Optional updates are a separate adoption step described in [UPDATES.md](UPDATES.md).
The current updater is a starting implementation with the limitations listed in
the README; adoption must test the intended app experience before shipping it.

## Verify before sharing

Run the app's checks, snapshot packaging, and `--version`. Verify installation
without development-only tools, then smoke-test on supported systems. After an
authorized release, verify the exact published `go install` command in a temporary
GOBIN before giving it to friends. Hosted CI and real app tests still need to run;
validating a generic template cannot substitute for them.
