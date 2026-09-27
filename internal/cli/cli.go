package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

const configName = "gokit.json"

type Config struct {
	Module string     `json:"module"`
	Web    string     `json:"web,omitempty"`
	Checks [][]string `json:"checks,omitempty"`
	Build  [][]string `json:"build,omitempty"`
	Watch  []string   `json:"watch,omitempty"`
	Dev    DevConfig  `json:"dev"`
}

type DevConfig struct {
	Args    []string `json:"args,omitempty"`
	Restart string   `json:"restart,omitempty"`
}

func Run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("go-kit — init · dev · ship\n\n  init --module github.com/you/app [--web] [--updates] [--python] [--yes] [dir]\n  dev [-- app arguments]\n  ship <version> [--dry-run] [--yes]\n\nRun a command with --help for details.")
		return nil
	}
	if args[0] == "--version" || args[0] == "-v" {
		version, commit := "dev", "unknown"
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				version = info.Main.Version
			}
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" {
					commit = s.Value
				}
				if s.Key == "vcs.modified" && s.Value == "true" {
					commit += "+dirty"
				}
			}
		}
		fmt.Printf("go-kit %s (%s %s %s/%s)\n", version, commit, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return nil
	}
	var err error
	switch args[0] {
	case "init":
		err = initProject(ctx, args[1:])
	case "dev":
		err = dev(ctx, args[1:])
	case "ship":
		err = ship(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q; use go-kit --help", args[0])
	}
	if errors.Is(err, flag.ErrHelp) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func flags(name string) *flag.FlagSet { return flag.NewFlagSet(name, flag.ContinueOnError) }
func step(label string)               { fmt.Println("\n→", label) }
func confirm(yes bool, question string) error {
	if yes {
		return nil
	}
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("%s; rerun with --yes to apply", question)
	}
	fmt.Print(question + " [y/N] ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		return errors.New("cancelled; no changes applied")
	}
	return nil
}
func command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, args[0], args[1:]...)
	c.Dir = dir
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	c.WaitDelay = 2 * time.Second
	return c
}
func run(ctx context.Context, dir string, args ...string) error {
	c := command(ctx, dir, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return nil
}
func output(ctx context.Context, dir string, args ...string) (string, error) {
	b, err := command(ctx, dir, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w\n%s", strings.Join(args, " "), err, b)
	}
	return strings.TrimSpace(string(b)), nil
}
func load(dir string) (Config, error) {
	var c Config
	f, err := os.Open(filepath.Join(dir, configName))
	if err != nil {
		return c, fmt.Errorf("run go-kit init first: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err = dec.Decode(&c); err != nil {
		return c, err
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return c, errors.New("gokit.json must contain one JSON object")
	}
	if !modulePattern.MatchString(c.Module) {
		return c, errors.New("module must be github.com/owner/repo (root executable)")
	}
	if c.Web != "" && (filepath.IsAbs(c.Web) || filepath.Clean(c.Web) != c.Web || strings.HasPrefix(c.Web, "..")) {
		return c, errors.New("web must be a project-relative directory")
	}
	if c.Dev.Restart == "" {
		c.Dev.Restart = "manual"
	}
	if c.Dev.Restart != "manual" && c.Dev.Restart != "auto" {
		return c, errors.New("dev.restart must be manual or auto")
	}
	for _, p := range c.Watch {
		if !relativePath(p) {
			return c, fmt.Errorf("watch path %q must be a project-relative file or directory (no globs)", p)
		}
	}
	for _, cmd := range append(append([][]string{}, c.Checks...), c.Build...) {
		if len(cmd) == 0 || cmd[0] == "" {
			return c, errors.New("checks and build must be nonempty argument arrays")
		}
	}
	return c, nil
}

var modulePattern = regexp.MustCompile(`^github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_-]+$`)

func exists(path string) bool { _, err := os.Lstat(path); return err == nil }
func prepareWeb(ctx context.Context, dir string, c Config) error {
	if c.Web == "" {
		return nil
	}
	if !exists(filepath.Join(dir, c.Web, "package-lock.json")) {
		return fmt.Errorf("missing %s/package-lock.json; run npm --prefix %s install and commit the lockfile", c.Web, c.Web)
	}
	if !exists(filepath.Join(dir, c.Web, "node_modules")) {
		return run(ctx, dir, "npm", "--prefix", c.Web, "ci")
	}
	return nil
}
func buildWeb(ctx context.Context, dir string, c Config) error {
	if err := prepareWeb(ctx, dir, c); err != nil {
		return err
	}
	if c.Web != "" {
		return run(ctx, dir, "npm", "--prefix", c.Web, "run", "build")
	}
	return nil
}

func relativePath(p string) bool {
	return p != "" && !filepath.IsAbs(p) && filepath.Clean(p) == p && p != ".." && !strings.HasPrefix(p, ".."+string(filepath.Separator)) && !strings.ContainsAny(p, "*?[")
}

// An explicit build list replaces the web shorthand, including an empty list.
func buildAssets(ctx context.Context, dir string, c Config) error {
	if c.Build == nil {
		return buildWeb(ctx, dir, c)
	}
	for _, args := range c.Build {
		if err := run(ctx, dir, args...); err != nil {
			return err
		}
	}
	return nil
}
