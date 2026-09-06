# go-kit

[Template](templates/app) · [Setup](SETUP.md) · [Releasing](RELEASING.md) · [Updates](UPDATES.md)

## Development

| Command | Action |
| --- | --- |
| `make run` | Run locally |
| `make run ARGS='...'` | Pass app arguments |
| `make check` | Run checks |
| `make build` or `make` | Build executable |
| `make release-check` | Local release preview; no publishing |

## Release

| Command | Action |
| --- | --- |
| `git tag -a <version> -m '<version>'` | Tag version |
| `git push origin HEAD` | Push commit |
| `git push origin <version>` | Trigger CI checks and publishing |

## Installation

| Command | Action |
| --- | --- |
| `go install github.com/you/myapp@<tag-or-commit>` | Fixed version or rollback |
| `go install github.com/you/myapp@main` | Current development revision |
| `go install github.com/you/myapp@latest` | Latest eligible version; one-time install |
| `myapp [arguments]` | Launch |
| `myapp --version` or `myapp -v` | Build identity; version snippet required |

## Optional updates

| Setting | Action |
| --- | --- |
| Unset / `export MYAPP_AUTO_UPDATE=0` | Fixed installation; default |
| `export MYAPP_AUTO_UPDATE=1` | Check newer release tags on launch |

| Behavior | Current template |
| --- | --- |
| Requirements | Update snippet; Go; macOS/Linux |
| Eligible builds | Non-prerelease tagged Go installations |
| Activation | Next launch |
| Failure | Existing executable retained |
| Startup delay | Up to about one minute |
| Release gate | Go tags; independent of release CI |
| Persistence | Shell startup setting |
| Pin / rollback | Disable updates, reinstall exact version |
