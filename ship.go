package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	versionRe = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	targets   = [][2]string{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "arm64"}, {"linux", "amd64"}}
)

const versionVar = "github.com/mcembalest/go-kit/kit.version"

// ship checks the repo, tests, cross-compiles, and (unless dryRun) tags and publishes a GitHub release
// with one archive per platform, checksums.txt, and install.sh. It returns the folder holding the artifacts.
func ship(ctx context.Context, dir, version string, dryRun, yes bool) (string, error) {
	p, err := detect(dir)
	if err != nil {
		return "", err
	}
	if !versionRe.MatchString(version) {
		return "", fmt.Errorf("version must look like v1.2.3, got %q", version)
	}
	repo, ok := strings.CutPrefix(p.module, "github.com/")
	if !ok || strings.Count(repo, "/") != 1 {
		return "", fmt.Errorf("module must be github.com/owner/repo to publish releases, got %s", p.module)
	}

	step("Check")
	if st, err := output(ctx, dir, "git", "status", "--porcelain"); err != nil || st != "" {
		if !dryRun {
			return "", errors.New("commit or stash changes first (git status is not clean)")
		}
		fmt.Println("  (dry run: working tree has uncommitted changes)")
	}
	if _, err := output(ctx, dir, "git", "rev-parse", "-q", "--verify", "refs/tags/"+version); err == nil {
		return "", fmt.Errorf("tag %s already exists", version)
	}
	if !dryRun {
		if out, _ := output(ctx, dir, "git", "ls-remote", "--tags", "origin", version); out != "" {
			return "", fmt.Errorf("tag %s already exists on origin", version)
		}
		if _, err := exec.LookPath("gh"); err != nil {
			return "", errors.New("publishing needs the GitHub CLI (gh), logged in")
		}
	}
	if err := p.buildWeb(ctx); err != nil {
		return "", err
	}
	if st, _ := output(ctx, dir, "git", "status", "--porcelain", "web"); st != "" && !dryRun {
		return "", errors.New("the web build changed committed files; commit web/dist and retry")
	}
	if p.python {
		if err := run(ctx, dir, "uv", "lock", "--check", "--project", "python"); err != nil {
			return "", fmt.Errorf("python/uv.lock is out of date (uv lock --project python): %w", err)
		}
	}
	if err := run(ctx, dir, "go", "vet", "./..."); err != nil {
		return "", err
	}
	if err := run(ctx, dir, "go", "test", "./..."); err != nil {
		return "", err
	}

	step("Build " + version)
	out, err := os.MkdirTemp("", "go-kit-ship-")
	if err != nil {
		return "", err
	}
	var artifacts []string
	for _, t := range targets {
		bin := filepath.Join(out, t[0]+"-"+t[1], p.name)
		cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", "-s -w -X "+versionVar+"="+version, "-o", bin, ".")
		cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+t[0], "GOARCH="+t[1])
		if err := cmd.Run(); err != nil {
			return out, fmt.Errorf("build %s/%s: %w", t[0], t[1], err)
		}
		archive := filepath.Join(out, fmt.Sprintf("%s_%s_%s_%s.tar.gz", p.name, version, t[0], t[1]))
		if err := tarGz(archive, map[string]string{p.name: bin, "README.md": filepath.Join(dir, "README.md")}); err != nil {
			return out, err
		}
		artifacts = append(artifacts, archive)
		fmt.Println("  " + filepath.Base(archive))
	}
	sums, err := checksums(artifacts)
	if err != nil {
		return out, err
	}
	install := filepath.Join(out, "install.sh")
	script := strings.NewReplacer("REPO", repo, "NAME", p.name, "VERSION", version).Replace(installScript)
	if err := os.WriteFile(install, []byte(script), 0o755); err != nil {
		return out, err
	}
	artifacts = append(artifacts, sums, install)
	fmt.Println("  checksums.txt\n  install.sh\n  in " + out)
	if dryRun {
		return out, nil
	}

	step("Publish")
	fmt.Printf("  tag %s at HEAD, push it to origin, and create a GitHub release for %s with %d files\n", version, repo, len(artifacts))
	if !yes && !confirm("Publish?") {
		return out, errors.New("not published")
	}
	if err := run(ctx, dir, "git", "tag", "-a", version, "-m", version); err != nil {
		return out, err
	}
	if err := run(ctx, dir, "git", "push", "origin", version); err != nil {
		_ = run(ctx, dir, "git", "tag", "-d", version)
		return out, err
	}
	args := append([]string{"gh", "release", "create", version, "--repo", repo, "--title", version, "--generate-notes"}, artifacts...)
	if err := run(ctx, dir, args...); err != nil {
		return out, fmt.Errorf("tag %s is pushed but the release failed; retry with: %s", version, strings.Join(args, " "))
	}
	fmt.Printf("\n  install: curl -fsSL https://github.com/%s/releases/latest/download/install.sh | sh\n", repo)
	fmt.Printf("  or:      go install %s@%s\n", p.module, version)
	return out, nil
}

func tarGz(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, src := range files {
		info, err := os.Stat(src)
		if err != nil {
			continue // optional files like README.md
		}
		h, _ := tar.FileInfoHeader(info, "")
		h.Name = name
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		r, err := os.Open(src)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, r)
		r.Close()
		if err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func checksums(files []string) (string, error) {
	var b strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%x  %s\n", sha256.Sum256(data), filepath.Base(f))
	}
	path := filepath.Join(filepath.Dir(files[0]), "checksums.txt")
	return path, os.WriteFile(path, []byte(b.String()), 0o644)
}

func confirm(question string) bool {
	fmt.Printf("%s [y/N] ", question)
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(ans)), "y")
}

func step(s string) { fmt.Println("→ " + s) }

// installScript is attached to each release; "latest/download/install.sh" installs the latest version.
const installScript = `#!/bin/sh
# Installs NAME VERSION from github.com/REPO into ~/.local/bin (set INSTALL_DIR to change).
set -eu
name=NAME
version=VERSION
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $(uname -m) in x86_64 | amd64) arch=amd64 ;; arm64 | aarch64) arch=arm64 ;; *) echo "unsupported CPU: $(uname -m)" >&2; exit 1 ;; esac
case $os in darwin | linux) ;; *) echo "unsupported OS: $os" >&2; exit 1 ;; esac
file=${name}_${version}_${os}_${arch}.tar.gz
url=https://github.com/REPO/releases/download/$version
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$url/$file" -o "$tmp/$file"
curl -fsSL "$url/checksums.txt" -o "$tmp/checksums.txt"
cd "$tmp"
grep " $file\$" checksums.txt > want
if command -v sha256sum > /dev/null; then sha256sum -c want > /dev/null; else shasum -a 256 -c want > /dev/null; fi
tar xzf "$file"
dir=${INSTALL_DIR:-$HOME/.local/bin}
mkdir -p "$dir"
mv "$name" "$dir/$name"
echo "installed $name $version to $dir/$name"
case ":$PATH:" in *":$dir:"*) ;; *) echo "add $dir to your PATH" ;; esac
`
