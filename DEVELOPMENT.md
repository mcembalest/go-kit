# Development

Build with `go build -o bin/go-kit .`; test with `go test -race ./...` and
`go vet ./...`. The CLI has no third-party Go dependencies. Source assets in
`internal/cli/assets` are embedded implementation details, copied into opted-in
apps; generated apps do not import go-kit.

## Lifecycle

`init` uses a fixed GitHub module path, root Go entry point, optional TypeScript
starter, and a small `gokit.json`. Existing Go modules are adopted by inserting
`gokitStartup()` into root `main()` and adding build metadata/release files.
Conflicting generated files and symlink destinations are rejected. Preview comes
before writes; `--yes` accepts it for agents. Root main.go is formatted by Go's
formatter. Existing asset pipelines are not guessed from directory names: configure
build/watch (or the web shorthand) explicitly when adopting an app. Review existing
app initialization/flags and generated platform targets.
A partial filesystem write failure can leave some setup files: inspect before
retrying. Without an existing .git, initialization creates a local Git repository and sets
origin from the module path. It creates no GitHub repository, commit, or push.
Create the matching GitHub repository and commit the reviewed project before shipping.

The web starter builds TypeScript with esbuild and checks it with TypeScript.
`init --web` runs npm in a temporary directory before writing the project, and
includes its lockfile and built assets. Development uses npm ci when node_modules
is absent; run npm ci after changing the lockfile yourself. Go embeds web/dist so
source installation needs only Go. The starter listens on loopback at port 8080;
ADDR overrides that. Browser reload is manual. No framework or database is imposed.

## Development primitives

`dev.restart` defaults to `manual`. Go-kit runs the build steps, builds Go, then
replaces its own process with the app. It keeps the same terminal descriptors and
foreground process group: keyboard input, resizing, Ctrl+C, exit status, and
terminal job control behave as they do when launching the executable directly.
There is no input proxy, terminal emulator, or ongoing supervisor. Exit/detach using
the app's normal controls and rerun `go-kit dev` to rebuild. The app remains
responsible for its raw-mode/alternate-screen cleanup, just as when run standalone.
Go-kit doesn't try to close an app-owned persistent session on detach.

Manual builds use one cached executable per project under the OS cache directory,
`go-kit/dev/<project-hash>/app`, so internal relaunches still have a valid path.
Build logs finish before terminal handoff. No source watching or background build
output interferes with an interactive session.

`dev.restart: "auto"` is for noninteractive servers and is selected by new browser
starters. Input is closed; stdout/stderr remain visible. Go-kit hashes inputs every
350ms, builds on changes, and only replaces the running app after a successful
build. Ctrl+C stops the supervised process group. An app that exits stays stopped
until the next source edit. Changing restart policy requires stopping/rerunning dev.
Persistent daemons or external services remain app-owned.

Both modes use `dev.args` unless explicit command-line arguments are supplied.
`go-kit dev --` explicitly clears saved arguments. In auto mode, config edits
(including arguments) trigger a rebuild/restart.

`build` is a list of native command argument arrays, run from the project root
before Go compilation in both dev and ship. Commands run sequentially; failure
stops the build. Omit the field to use the existing `web` npm build shorthand;
explicit `[]` disables that shorthand. An explicit list owns all asset preparation,
including dependency setup. No shell expansion occurs: use an app-owned script
when needed. TypeScript, Python via uv, and other tools use this same mechanism.

Auto mode watches Go files, module/config files, web inputs, and local Go embed
inputs discovered with `go list`. Directory/glob embeds watch the covering directory
so newly added matches trigger builds. `watch` adds relative files/directories for
other inputs; paths may not escape the project and don't accept globs. Symlinks are
not followed. Git and node_modules are always skipped. Hidden directories, bin,
and dist are skipped by default (avoiding generated-output loops), but can be
included with explicit watch paths. Keep build outputs outside watched source
paths. Embed discovery is refreshed after successful builds.

Example project settings (adapt the commands to the app):

```json
{
  "module": "github.com/you/app",
  "build": [["npm", "--prefix", "ui", "run", "build"]],
  "watch": ["ui/src", "ui/package.json", "assets"],
  "dev": {"args": ["--no-open"], "restart": "auto"}
}
```

`ship` verifies a clean committed root, matching module, branch, and unused version.
For publication it also verifies origin and GitHub authentication. The web shorthand
reinstalls UI dependencies from the lockfile; explicit build commands manage their
own setup. Asset builds, Go race tests/vet, and checks in
`gokit.json` run before packaging. Any changed/untracked source files block release.
Checks are argument arrays, not shell strings; use native tools or app-owned scripts.

Dry-run uses GoReleaser snapshot packaging and never creates a tag. Its filenames
use the current checkout's snapshot version, not the proposed version. Publication
creates a local annotated tag, packages using GoReleaser with publishing disabled,
then asks before atomically pushing the current branch and tag. GitHub CLI creates
the release with archives, checksums and generated notes. There is no remote CI
completion gate: tests execute locally before publication; generated CI also runs
Go checks independently on pushes/PRs. Keep custom builds and extra checks in your app's CI too.

Local failures remove the newly created tag before a push. An ambiguous push
retains the tag if the remote cannot be checked. After a confirmed push, tags are
never deleted automatically. If release creation fails, inspect GitHub and finish
with gh release create/upload; do not move or reuse published tags.

GoReleaser must retain the generated tar.gz and checksums.txt output contract.
Adjust its OS/architecture matrix and CGO setting for the app. Archives are built
for macOS/Linux amd64/arm64 by default; compilation does not prove runtime support.
App-owned assets/licenses and external runtimes must be documented before release.
No Apple signing/notarization or hosted deployment is configured.

## Optional updates

`init --updates` includes app-owned Go update code and tests. Its environment name
is the uppercase app name with hyphens changed to underscores, plus _AUTO_UPDATE.
Users enable it with value 1; it is off otherwise. Persist the setting in the user's
shell configuration. An exact-version install does not override an existing opt-in.

The updater queries Go's latest module version, accepts only a newer non-prerelease
tag, stages a Go installation, validates module/version, and replaces the executable.
A lock serializes updater processes. Updates take effect next launch; existing
sessions continue. Network/build failures retain the installed executable. Go must
remain installed and checks can delay startup about one minute. Checkout, archive,
prerelease and development builds are excluded. Go tags may be visible before CI
finishes. External tools and credentials are never updated. Disable opt-in before
pinning or rolling back. Review placement for apps with internal relaunches.

## Validation

Tests use disposable Go projects and never publish. Optional integration tests:
`GOKIT_WEB_TEST=1 go test ./internal/cli -run TestWeb` exercises npm/UI scaffolding;
`GOKIT_RELEASE_TEST=1 go test ./internal/cli -run TestShipPreview` exercises the
installed GoReleaser. Neither touches product repositories. Actual authenticated
publication still requires an explicitly authorized release rehearsal.

Local validation on macOS passed with Go 1.27: race tests, Go vet, real snapshot
packaging, Go-only and browser source installs, adoption, live rebuilds, and failure
recovery. Publication tests use a local bare Git remote and a fake GitHub CLI;
no real release was published. Go 1.22 test binaries abort on this host's macOS
loader (missing LC_UUID), so minimum-version runtime validation is delegated to
the Linux CI job and has not yet run in GitHub.

Terminal tests use the system script utility to create a pseudo-terminal and
exercise input, window-size changes, resize signals, and Ctrl+C. Native rendering
in specific terminal apps still needs hands-on use.
