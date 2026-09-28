package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// project is everything go-kit needs, read from the repo itself: no go-kit files.
type project struct {
	dir    string
	module string // github.com/owner/repo
	name   string // binary name: last element of the module path
	web    bool   // web/package.json with a build script: built before Go, output embedded by the app
	python bool   // python/pyproject.toml: embedded uv project (see kit.Python)
}

func detect(dir string) (project, error) {
	p := project{dir: dir}
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return p, errors.New("no go.mod here; run go-kit from the app's root")
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
			p.module = strings.Trim(f[1], `"`)
		}
	}
	if p.module == "" {
		return p, errors.New("go.mod has no module line")
	}
	p.name = filepath.Base(p.module)
	p.web = exists(filepath.Join(dir, "web", "package.json"))
	p.python = exists(filepath.Join(dir, "python", "pyproject.toml"))
	return p, nil
}

// buildWeb runs the web build (installing locked dependencies first if needed).
func (p project) buildWeb(ctx context.Context) error {
	if !p.web {
		return nil
	}
	if !exists(filepath.Join(p.dir, "web", "node_modules")) {
		if err := run(ctx, p.dir, "npm", "--prefix", "web", "ci"); err != nil {
			return err
		}
	}
	return run(ctx, p.dir, "npm", "--prefix", "web", "run", "build")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func run(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func output(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
