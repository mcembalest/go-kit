# go-kit

Personal templates for Go apps with optional TypeScript UI. Friends install each app directly; they don't need go-kit. Other languages stay app-owned; use uv for Python.

## Setup

Copy the [app template](templates/app) into your app's repo. Follow [setup](SETUP.md).

## Develop

Run in the app repo:

| Command | Action |
| --- | --- |
| `make run` | Run locally |
| `make run ARGS='...'` | Pass app arguments |
| `make check` | Run checks |
| `make build` or `make` | Build executable |
| `make release-check` | Check and preview release archives locally |

Release: `git tag -a <version> -m '<version>'` → `git push origin HEAD` → `git push origin <version>`. Tag pushes trigger CI checks and publishing.

## Install and use

Replace `github.com/you/myapp` and `myapp` with your app. Put Go's install directory on PATH.

| Command | Action |
| --- | --- |
| `go install github.com/you/myapp@<tag-or-commit>` | Install a fixed build; use an older tag to roll back |
| `go install github.com/you/myapp@main` | Install current development revision |
| `go install github.com/you/myapp@latest` | Install latest eligible version once |
| `myapp [arguments]` | Launch |
| `myapp --version` or `myapp -v` | Identify build after adopting version snippet |

## Optional updates

Requires adopting the [update template](UPDATES.md). Off by default.

| Setting | Action |
| --- | --- |
| `export MYAPP_AUTO_UPDATE=1` | Check for newer release tags on launch |
| `export MYAPP_AUTO_UPDATE=0` | Keep installed version fixed |

Requires Go; updates apply next launch. Current template may delay startup up to about a minute and follows tags before release CI completes. Development, prerelease, and archive builds are excluded. Disable updates before pinning or rolling back; change shell startup settings to persist the choice.

[Setup](SETUP.md) · [Releasing](RELEASING.md) · [Update details](UPDATES.md)
