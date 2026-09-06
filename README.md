# go-kit

My personal toolkit for setting up, checking, and sharing apps. Friends install
Go and then install each app directly with `go install`; they never run go-kit.

## Default shape

- **Go:** entry point, launcher, local tools, backend, and source installation.
- **TypeScript:** browser UI when needed, built ahead of installation and embedded
  in Go. Apps without browser UI do not need TypeScript or Node.
- **Other languages:** optional, app-owned components; use uv for Python tooling.
  Anything required at runtime must be bundled deliberately or documented as an
  external prerequisite. A Go launcher alone does not bundle another runtime.

Start with root `go.mod` and `main.go`. Reuse the app's existing layout and tools
where possible. The base template is Go-only; [setup](SETUP.md) explains where UI
builds and other checks join the same commands.

## My command surface

Copy [the template](templates/app) into an app; run these **in that app's repo**.
There is no go-kit command or installer. Setup is a one-time, manual copy/adapt step.

| Command | Meaning |
| --- | --- |
| `make run` | Run locally; include the app's UI build when applicable |
| `make run ARGS='...'` | Run with app-specific arguments |
| `make check` | Go race tests and vet, plus the app's UI/other checks |
| `make build` | Build `bin/myapp`; include current UI assets when applicable |
| `make` | Same as `make build` |
| `make release-check` | Checks and GoReleaser snapshot archives in `dist/`; does not publish |

Publishing uses ordinary Git commands after checks pass:

```sh
git tag -a v0.1.0 -m 'v0.1.0'
git push origin HEAD
git push origin v0.1.0
```

Use an intended, unused version. Pushing the tag triggers CI checks and GoReleaser
publishing. There is no `make release` or custom release CLI.
Development dependencies are installed with each app's documented native tools;
there is currently no shared `setup`, `doctor`, or dependency-install command.

## Friend command surface

`github.com/you/myapp`, `myapp`, and the versions below are placeholders.
Go's installation directory must be on PATH. Other runtime requirements are
listed by the app; friends do not need Make, GoReleaser, or UI build tools.

| Command | Meaning |
| --- | --- |
| `go install github.com/you/myapp@v0.1.0` | Install an exact version |
| `go install github.com/you/myapp@<commit>` | Install a specific development revision |
| `go install github.com/you/myapp@main` | Install the current development branch; rerun to refresh |
| `go install github.com/you/myapp@latest` | Install Go's latest eligible version once; not an auto-update setting |
| `myapp [arguments]` | Launch the app; arguments belong to that app |
| `myapp --version` or `myapp -v` | Report installed build identity, once the version snippet is wired in |

For rollback, turn off automatic updates if enabled, then reinstall the desired
exact version. App-specific flags and subcommands belong in the app's README.

## Optional update surface

This is an **optional template to adopt and test**, not behavior installed in any
app by go-kit. Fixed installations are the default.

| Setting | Meaning |
| --- | --- |
| `export MYAPP_AUTO_UPDATE=1` | Opt into checking newer non-prerelease tags on app launch |
| `export MYAPP_AUTO_UPDATE=0` | Disable automatic updates and keep the installed version fixed |

Choose a unique variable per app. Exports affect the current shell and its child
processes; change the shell startup file to persist the choice. An exact-version
install does not cancel an existing opt-in setting.

Current behavior: requires Go, supports version-tagged Go installations on
macOS/Linux, and applies successful updates **next launch**. Failures retain the
existing executable. Checks may delay startup by about a minute. Development,
prerelease, and archive builds are excluded. Updates follow Go module tags, which
can become visible before release CI finishes. External dependencies are not updated.
There are no `myapp update` or `myapp pin` commands. See [update details](UPDATES.md).

## What this toolkit covers

Copyable build/check/release configuration, build identification, dependency notes,
exact-version installation, rollback instructions, and optional update behavior.
It does not currently provide persistent app settings, diagnostic bundles, or
background update services. Add those only when testing real apps demonstrates a need.

[Setup](SETUP.md) · [Releasing](RELEASING.md) · [Updates](UPDATES.md)
