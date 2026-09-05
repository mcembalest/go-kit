# Release and share an app

Run this in the app repository after [setup](SETUP.md). Go installation fetches
source from that repository; GitHub release archives are an additional option.

1. Review and commit the intended changes; require a clean working tree.
2. Run `make release-check`. Smoke-test the app and note external dependency versions.
3. When authorized to publish, choose an unused tag and explicitly push the commit
   and tag. For example, with an actual intended version substituted:

   ```sh
   git tag -a v0.1.0 -m 'v0.1.0'
   git push origin HEAD
   git push origin v0.1.0
   ```

4. CI checks the tagged commit before GoReleaser publishes archives, checksums,
   and release notes. Add known limitations and tested external tool versions.
5. Verify the exact installation command, then share it with friends.

Example commands use placeholders, not existing releases:

```sh
go install github.com/you/myapp@v0.1.0
myapp --version
myapp
```

Put `go env GOBIN` on PATH if set, otherwise `$(go env GOPATH)/bin`.
For development, `@main` installs the current branch revision; an exact commit
pins a development build. Neither form automatically updates itself. For releases,
use independent `v0.x.y` tags per app, with explicit prerelease suffixes if needed.
Reinstall an older version to roll back, disabling any opt-in updates first.
Never move or reuse a published tag.

A pushed tag becomes available to Go independently of GitHub Release success.
Run checks before pushing it, especially when users have opted into updates.
Private repos additionally require Git access and Go private-module configuration.

## Build identity and dependencies

Ask for `myapp --version`, OS, reproduction steps, and relevant external tool
versions. Source installations embed a module version or pseudo-version. Checkout
builds include VCS state when available; release archives include an injected
version and commit. Missing metadata stays unknown. Snapshots are local previews,
not Go module versions, and a dirty snapshot's commit only identifies its base.

Archives include the app's dependency notes and existing license files. Add any
required bundled-asset notices. Choose the app's license and confirm redistribution
requirements in that repository. External programs and credentials are not
implicitly bundled. No paid Apple signing/notarization is configured; downloaded
macOS apps may still encounter platform security prompts.

[GoReleaser OSS](https://goreleaser.com/getting-started/quick-start/) handles packaging
and publishing through [GitHub Actions](https://goreleaser.com/customization/ci/actions/).
It is a developer tool; friends do not need it.
