package main

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// autoUpdate is optional, supports macOS/Linux, and only updates go-installed
// stable versions. The new executable is used on the NEXT launch.
func autoUpdate(module, optIn string) {
	if os.Getenv(optIn) != "1" {
		return
	}
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" || arg == "--help" || arg == "-h" {
			return
		}
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Path != module || info.Main.Replace != nil {
		return
	}
	// Checkout builds (including GoReleaser archives) aren't managed by go install.
	for _, setting := range info.Settings {
		if setting.Key == "vcs" {
			return
		}
	}
	if _, ok := stableVersion(info.Main.Version); !ok {
		return
	}
	path, err := os.Executable()
	if err == nil {
		path, err = filepath.EvalSymlinks(path)
	}
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		err = installUpdate(ctx, module, info.Main.Version, path)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Automatic update skipped; continuing with this build:", err)
	}
}

func installUpdate(ctx context.Context, module, current, path string) error {
	lock, err := os.OpenFile(path+".update.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil // Another launcher is checking; don't delay this one.
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	// A concurrently started old process must not overwrite a newer installed build.
	// The lock file stays in place so all processes lock the same inode.
	installed, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	if installed.Main.Path != module {
		return fmt.Errorf("installed module does not match %s", module)
	}
	if installed.Main.Version != current {
		return nil
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", module+"@latest")
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("query Go module: %w", err)
	}
	var latest struct{ Version string }
	if err := json.Unmarshal(out, &latest); err != nil {
		return err
	}
	if !newerStable(latest.Version, current) {
		return nil
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), ".go-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	cmd = exec.CommandContext(ctx, "go", "install", module+"@"+latest.Version)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOBIN="+dir, "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go install: %w: %s", err, out)
	}
	candidate := filepath.Join(dir, filepath.Base(module))
	built, err := buildinfo.ReadFile(candidate)
	if err != nil {
		return err
	}
	if built.Main.Path != module || built.Main.Version != latest.Version {
		return fmt.Errorf("downloaded build does not match requested module/version")
	}
	if err := os.Rename(candidate, path); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Installed", latest.Version+"; it will be used next launch.")
	return nil
}

func stableVersion(s string) ([3]uint64, bool) {
	var version [3]uint64
	if !strings.HasPrefix(s, "v") {
		return version, false
	}
	parts := strings.Split(s[1:], ".")
	if len(parts) != 3 {
		return version, false
	}
	for i, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return version, false
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return version, false
		}
		version[i] = n
	}
	return version, true
}

func newerStable(candidate, current string) bool {
	a, aOK := stableVersion(candidate)
	b, bOK := stableVersion(current)
	if !aOK || !bOK {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
