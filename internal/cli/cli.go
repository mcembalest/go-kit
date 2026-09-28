// Package cli is the go-kit command: build, run, and ship Go apps.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/mcembalest/go-kit/kit"
)

const usage = `go-kit: build, run, and ship a Go app from its root folder

  go-kit dev [app args]                 build (web UI first, if web/package.json) and run the app
  go-kit ship vX.Y.Z [--dry-run] [-y]   check, test, cross-compile, and publish a GitHub release
                                        with archives for macOS/Linux, checksums, and install.sh
  go-kit -v                             go-kit version

An app needs only go.mod with module github.com/owner/repo. Optional:
  web/package.json         "build" script; its output is embedded by the app (//go:embed)
  python/pyproject.toml    uv project with main.py, run by kit.Python (github.com/mcembalest/go-kit/kit)

Installs:  go install github.com/owner/repo@main                       (dev builds, needs Go)
           curl -fsSL https://github.com/owner/repo/releases/latest/download/install.sh | sh
`

// Main runs go-kit with the process arguments.
func Main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cli(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "go-kit:", err)
		os.Exit(1)
	}
}

func cli(ctx context.Context, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}
	switch args[0] {
	case "dev":
		if len(args) > 1 && args[1] == "--" {
			args = args[1:]
		}
		return dev(ctx, dir, args[1:])
	case "ship":
		var version string
		dry, yes := false, false
		for _, a := range args[1:] {
			switch a {
			case "--dry-run":
				dry = true
			case "-y", "--yes":
				yes = true
			default:
				if version != "" {
					return fmt.Errorf("unexpected argument %s", a)
				}
				version = a
			}
		}
		_, err := ship(ctx, dir, version, dry, yes)
		return err
	case "-v", "--version":
		fmt.Println("go-kit", kit.Version())
		return nil
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	}
	return fmt.Errorf("unknown command %s (go-kit -h)", args[0])
}
