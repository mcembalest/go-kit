package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$`)

func ship(ctx context.Context, args []string) error {
	dry, yes := false, false
	version := ""
	for _, a := range args {
		switch a {
		case "--dry-run":
			dry = true
		case "--yes":
			yes = true
		case "--help", "-h":
			fmt.Println("go-kit ship <version> [--dry-run] [--yes]\n\nTest and package; then push current branch/tag and publish a GitHub release.\n--dry-run builds snapshot archives without creating tags, pushing, or publishing.\nRequires a clean committed Git root, GoReleaser, and GitHub CLI for publishing.")
			return nil
		default:
			if version != "" || !tagPattern.MatchString(a) {
				return errors.New("usage: go-kit ship v0.1.0 [--dry-run] [--yes]")
			}
			version = a
		}
	}
	if version == "" {
		return errors.New("provide a version, for example go-kit ship v0.1.0 --dry-run")
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	c, err := load(dir)
	if err != nil {
		return err
	}
	root, err := output(ctx, dir, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	real, _ := filepath.EvalSymlinks(dir)
	if root != real {
		return errors.New("run ship from the Git repository root")
	}
	if err := clean(ctx, dir); err != nil {
		return err
	}
	module, err := output(ctx, dir, "go", "list", "-m")
	if err != nil {
		return err
	}
	if module != c.Module {
		return errors.New("gokit.json module differs from go.mod")
	}
	branch, err := output(ctx, dir, "git", "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return errors.New("check out a branch before shipping")
	}
	if _, err := output(ctx, dir, "git", "rev-parse", "--verify", "HEAD"); err != nil {
		return errors.New("commit the project before shipping")
	}
	if _, err := output(ctx, dir, "git", "rev-parse", "-q", "--verify", "refs/tags/"+version); err == nil {
		return fmt.Errorf("tag %s already exists; choose a new version", version)
	}
	if !dry {
		remote, err := output(ctx, dir, "git", "remote", "get-url", "origin")
		if err != nil {
			return err
		}
		repo := strings.TrimPrefix(c.Module, "github.com/")
		if remote != "https://github.com/"+repo+".git" && remote != "https://github.com/"+repo && remote != "git@github.com:"+repo+".git" {
			return errors.New("origin must match gokit.json's GitHub repository")
		}
		found, err := output(ctx, dir, "git", "ls-remote", "--tags", "origin", "refs/tags/"+version)
		if err != nil {
			return err
		}
		if found != "" {
			return fmt.Errorf("remote tag %s already exists", version)
		}
		if err := run(ctx, dir, "gh", "auth", "status"); err != nil {
			return err
		}
	}
	step("Validate " + version)
	if c.Web != "" && c.Build == nil {
		if err := run(ctx, dir, "npm", "--prefix", c.Web, "ci"); err != nil {
			return err
		}
	}
	if err := buildAssets(ctx, dir, c); err != nil {
		return err
	}
	commands := [][]string{{"go", "test", "-race", "./..."}, {"go", "vet", "./..."}}
	commands = append(commands, c.Checks...)
	for _, cmd := range commands {
		if err := run(ctx, dir, cmd...); err != nil {
			return err
		}
	}
	if err := clean(ctx, dir); err != nil {
		return fmt.Errorf("checks changed project files; review and commit them: %w", err)
	}
	if err := run(ctx, dir, "goreleaser", "check"); err != nil {
		return err
	}
	step("Package")
	if dry {
		if err := run(ctx, dir, "goreleaser", "release", "--snapshot", "--clean", "--skip=publish"); err != nil {
			return err
		}
		fmt.Printf("\nPreview ready in dist/. Requested tag: %s (not created).\nSnapshot filenames use GoReleaser's current-checkout version.\nNo push or publication.\n", version)
		return nil
	}
	if err := run(ctx, dir, "git", "tag", "-a", version, "-m", version); err != nil {
		return err
	}
	pushed := false
	defer func() {
		if !pushed {
			_, _ = output(context.Background(), dir, "git", "tag", "-d", version)
		}
	}()
	if err := run(ctx, dir, "goreleaser", "release", "--clean", "--skip=publish"); err != nil {
		return err
	}
	if err := clean(ctx, dir); err != nil {
		return err
	}
	archives, err := filepath.Glob(filepath.Join(dir, "dist", "*.tar.gz"))
	if err != nil {
		return err
	}
	if len(archives) == 0 {
		return errors.New("no .tar.gz archives in dist; initial ship supports GoReleaser tar.gz archives")
	}
	checksums := filepath.Join(dir, "dist", "checksums.txt")
	if !exists(checksums) {
		return errors.New("missing dist/checksums.txt")
	}
	step("Publish " + c.Module + "@" + version)
	fmt.Printf("  branch   %s\n  archives %d\n", branch, len(archives))
	if err := confirm(yes, "Push this branch/tag and publish the release?"); err != nil {
		return err
	}
	if err := run(ctx, dir, "git", "push", "--atomic", "origin", "HEAD:refs/heads/"+branch, "refs/tags/"+version); err != nil {
		// A failed response may be ambiguous. Retain a tag found remotely.
		remote, e := output(context.Background(), dir, "git", "ls-remote", "--tags", "origin", "refs/tags/"+version)
		pushed = e != nil || remote != ""
		return err
	}
	pushed = true
	cmd := []string{"gh", "release", "create", version, "--repo", strings.TrimPrefix(c.Module, "github.com/"), "--verify-tag", "--generate-notes", "--title", version}
	if strings.Contains(version, "-") {
		cmd = append(cmd, "--prerelease")
	}
	cmd = append(cmd, archives...)
	cmd = append(cmd, checksums)
	if err := run(ctx, dir, cmd...); err != nil {
		return fmt.Errorf("tag is pushed but release publication failed; inspect GitHub Releases before retrying with gh release create/upload: %w", err)
	}
	fmt.Printf("\nShipped.\n\n  go install %s@%s\n\nhttps://github.com/%s/releases/tag/%s\n", c.Module, version, strings.TrimPrefix(c.Module, "github.com/"), version)
	return nil
}
func clean(ctx context.Context, dir string) error {
	status, err := output(ctx, dir, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if status != "" {
		return errors.New("working tree is not clean; review and commit changes before shipping")
	}
	return nil
}
