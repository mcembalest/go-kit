# Pinned installs and optional updates

Pinned is the default. An exact `go install` version stays installed until the
user explicitly installs another version or enables automatic updates.
The optional Go template uses the standard `go` command for module resolution,
checksum verification, and compilation. There is no additional Go dependency.

## Friend choices

For a fixed version (substitute an existing tag):

```sh
export MYAPP_AUTO_UPDATE=0
go install github.com/you/myapp@v0.1.0
```

Remove any earlier `..._AUTO_UPDATE=1` setting from the shell startup file, or
change it to `0`, to keep this choice across new terminals. Reinstalling an exact
tag alone does not override a previously enabled environment setting.

To opt into stable-tag updates after the product adopts the optional template:

```sh
export MYAPP_AUTO_UPDATE=1
```

Use a distinct variable for each app. Add the line to the shell startup file for persistence.
Start from an exact stable `v0.x.y` installation. Prerelease and development builds
remain fixed; use an explicit install command to change them. No schedule or shell
startup file is changed by go-kit itself.

On launch, the app asks Go for the latest version, and only installs it if it is
an ordinary `vMAJOR.MINOR.PATCH` tag newer than the current version. A successful
update is announced and used on the **next launch**. The current session continues
running its original build. `--version` therefore still identifies that session's
build correctly; checking the newly installed executable reports the new build.
Checks can delay startup by up to approximately one minute. Missing Go, network
errors, build failures, and unwritable installation paths leave the current build
in use. Other running sessions are not restarted. No background service is added.

Turn the environment setting off and reinstall a previous tag to roll back.
Automatic updates only change the product executable: external runtimes,
credentials, and example environments retain their separate setup/update policies.

## Product adoption

Copy `templates/optional/autoupdate.go` and `autoupdate_test.go` into the product
root. After the early help/version handling and before product startup, call:

```go
autoUpdate("github.com/you/myapp", "MYAPP_AUTO_UPDATE")
```

Keep module paths fixed in source. This template is specifically for the initial
root-executable, macOS/Linux projects; it is not a general installer for arbitrary
packages. It skips help and version requests. Call it only at user-facing startup, not
internal relaunches or worker entry points.
It also skips checkout builds, GoReleaser archives with VCS metadata, prereleases,
and replacements. Plain `go install ...@v0.x.y` builds are the supported update path.
Archive users can install a new archive manually or switch to Go installation.

The implementation stages builds in the executable's directory, verifies the
built module/version, then atomically renames the successful build into place.
A nonblocking file lock prevents concurrent update attempts; its small
`<executable>.update.lock` file remains next to the executable. Don't delete it
while launchers are checking. No privileges are requested and no system-wide
installation path is assumed.

Before adopting, run `go test -race ./...` and verify opt-in/off launches against
an isolated installation. Add these choices to the product's install instructions.
Each app owns adoption and testing of this behavior. Go-kit does not install
updates on its owner's or friends' machines.

## Release implications and tool choice

Go's `@latest` resolution follows module tags, not successful GitHub Releases.
An opted-in installation can therefore see a pushed stable tag before its release
workflow finishes. Run checks on the intended commit **before pushing a stable
tag**, and never move tags. Public module proxies may also delay visibility.
For a stricter requirement that only completed GitHub Releases can update users,
use a release-asset updater instead of this source-install recipe.

[Go's module reference](https://go.dev/ref/mod#go-install) documents versioned
installation and [version queries](https://go.dev/ref/mod#version-queries).
[creativeprojects/go-selfupdate](https://github.com/creativeprojects/go-selfupdate)
is an established option for replacing executables from GitHub release archives,
including GoReleaser checksum validation. Evaluated v1.6.0: it requires Go 1.25.12
and adds provider/archive dependencies. Defer it while friends already have Go;
revisit if archive-based automatic updates become necessary. GoReleaser itself
packages and publishes releases; it does not make installed apps auto-update.
