# go-kit

## Install

```sh
go install github.com/mcembalest/go-kit@main
```

## Commands

| Command | Action |
| --- | --- |
| `go-kit init --module github.com/you/app [dir]` | Preview and create a Go project |
| `go-kit init [dir]` | Preview and adopt a root Go executable |
| `go-kit init --web --module github.com/you/app [dir]` | Create Go + TypeScript browser app |
| `go-kit init --updates ...` | Include optional app-update support; off by default |
| `go-kit init --python ...` | Include an embedded Python worker run with uv |
| `go-kit init --yes ...` | Apply without prompting |
| `go-kit dev [-- arguments]` | Build/run; explicit arguments replace saved arguments |
| `go-kit ship <version> --dry-run` | Test and package snapshots; no tag/push/publication |
| `go-kit ship <version>` | Test, package, confirm, push branch/tag, publish GitHub release |
| `go-kit ship <version> --yes` | Publish without prompting |
| `go-kit --version` / `-v` | Toolkit build identity |
| `go-kit --help` / `-h` | Command list |
| `go-kit <command> --help` / `-h` | Command help |

## Project settings · `gokit.json`

| Key | Default / meaning |
| --- | --- |
| `module` | Required `github.com/owner/repo`; root executable |
| `web` | None; UI directory with locked npm dependencies and a `build` script |
| `build` | Argument arrays before Go builds in dev/ship; overrides the web build shorthand |
| `watch` | Extra relative input files/directories; no globs; auto mode only |
| `dev.args` | Saved app arguments; default `[]`; `dev --` clears them |
| `dev.restart` | `manual` default: direct terminal handoff; rerun dev to rebuild |
| `dev.restart: "auto"` | Noninteractive server watch/restart; generated browser starter default |
| `checks` | None; extra checks as argument arrays, after Go race tests and vet |

## App surface

| Command / setting | Action |
| --- | --- |
| `go install github.com/you/app@<tag-or-commit>` | Fixed installation or rollback |
| `go install github.com/you/app@main` | Current development revision |
| `go install github.com/you/app@latest` | Latest eligible version; one-time install |
| `app --version` / `app -v` | Build identity |
| `ADDR=127.0.0.1:8080` | Default browser-starter address |
| `<APP>_AUTO_UPDATE=1` | Opt in to newer release tags; requires `init --updates` |
| `<APP>_AUTO_UPDATE=0` / unset | Fixed installation; default |

## Requirements / behavior

| Area | Current support |
| --- | --- |
| Platform | macOS / Linux; Go 1.22+; Git |
| Terminal development | Manual mode; native input, resizing, signals, and app exit status |
| Watched inputs | Go/module/config, embedded assets, UI sources, and explicit watch paths |
| UI development | Node 24/npm; committed browser bundle; manual browser refresh |
| Python worker | uv at runtime; `python/` embedded; JSON lines over stdin/stdout; locked with `uv.lock` |
| Initialization | Local Git/origin if absent; no GitHub repo, commit, or push |
| Shipping | Clean Git root; GoReleaser OSS 2.18.0; GitHub CLI/auth for publication |
| Publication | Explicit `ship`; tag pushes alone do not publish |
| Release failure | Pre-push local tag removed; post-push failures require inspecting GitHub |
| Updates | Tagged Go installations only; next launch; up to one-minute startup delay |
| Pin / rollback | Disable updates first, then install an exact version |
| App dependencies | App-owned `DEPENDENCIES.md`; not implicitly bundled or updated |
| Remote deployment | GitHub releases / Go installation; no hosted-service deployment |

[Development and limitations](DEVELOPMENT.md)
