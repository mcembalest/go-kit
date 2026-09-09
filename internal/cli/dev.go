package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func dev(ctx context.Context, args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println("go-kit dev [-- app arguments]\n\nBuild and run. dev.restart=manual hands the terminal to the app; rerun to rebuild.\nUse dev.restart=auto for server watching. Explicit arguments replace dev.args.")
		return nil
	}
	overrideArgs := len(args) > 0
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	c, err := load(dir)
	if err != nil {
		return err
	}
	if !overrideArgs {
		args = append([]string{}, c.Dev.Args...)
	}
	if c.Dev.Restart == "manual" {
		return manualDev(ctx, dir, c, args)
	}
	embeds, err := embeddedFiles(ctx, dir)
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "go-kit-dev-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	var child *exec.Cmd
	var done chan error
	stop := func() {
		if child == nil {
			return
		}
		_ = syscall.Kill(-child.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
			<-done
		}
		child = nil
		done = nil
	}
	defer stop()
	build := func() {
		step("Build")
		fresh, err := load(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if fresh.Dev.Restart != "auto" {
			fmt.Fprintln(os.Stderr, "Restart policy changed; stop and rerun go-kit dev to apply it.")
			return
		}
		c = fresh
		if !overrideArgs {
			args = append([]string{}, c.Dev.Args...)
		}
		if err := buildAssets(ctx, dir, c); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		binary := filepath.Join(temp, "app-next")
		if err := run(ctx, dir, "go", "build", "-o", binary, "."); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if discovered, e := embeddedFiles(ctx, dir); e == nil {
			embeds = discovered
		} else {
			fmt.Fprintln(os.Stderr, e)
		}
		stop()
		child = exec.Command(binary, args...)
		child.Dir = dir
		// Automatic mode is for noninteractive servers; terminal apps use manual.
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			child = nil
			return
		}
		done = make(chan error, 1)
		cmd := child
		ch := done
		go func() { ch <- cmd.Wait() }()
		fmt.Println("→ Running · watching for changes · Ctrl+C to stop")
	}
	last, err := fingerprint(dir, c, embeds)
	if err != nil {
		return err
	}
	build()
	// Keep the pre-build fingerprint: edits during a build must trigger another.
	tick := time.NewTicker(350 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if err != nil {
				fmt.Fprintln(os.Stderr, "App stopped:", err)
			}
			// Clean up descendants even if the parent exited.
			if child != nil {
				_ = syscall.Kill(-child.Process.Pid, syscall.SIGTERM)
			}
			child = nil
			done = nil
		case <-tick.C:
			next, err := fingerprint(dir, c, embeds)
			if err != nil {
				return err
			}
			if next != last {
				last = next
				build()
			}
		}
	}
}

// Replace go-kit instead of proxying a terminal: the app keeps the foreground
// process group, file descriptors, resize notifications, signals, and exit status.
func manualDev(ctx context.Context, dir string, c Config, args []string) error {
	step("Build")
	if err := buildAssets(ctx, dir, c); err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(canonical))
	folder := filepath.Join(cache, "go-kit", "dev", fmt.Sprintf("%x", sum[:16]))
	if err := os.MkdirAll(folder, 0700); err != nil {
		return err
	}
	binary := filepath.Join(folder, "app")
	if err := run(ctx, dir, "go", "build", "-o", binary, "."); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return syscall.Exec(binary, append([]string{binary}, args...), os.Environ())
}
