# Scope

- go-kit builds, runs, and ships Go apps: `dev` and `ship`, plus the small `kit` package apps import.
- Apps carry no go-kit files. Read what's needed from the repo: go.mod, web/package.json, python/pyproject.toml.
- Nothing is generated into app repos. Release artifacts are built in a temp folder.
- Two installs per app: `go install …@main` (dev builds, needs Go) and GitHub release archives + install.sh (no Go).
- Apps must run for people without dev tools: kit.Python downloads a pinned, checksummed uv when missing.
- Usage lives in `go-kit -h`. README is install plus `-h`.
- Keep repo roots minimal: a thin main.go (embeds + one call), go.mod/go.sum, README.md; code in internal/, plus web/ and python/ when used.
- Test with disposable projects. Do not push, tag, or publish without authorization.
