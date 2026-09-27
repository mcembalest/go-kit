package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// dev builds the web UI (if any) and the app, then replaces go-kit with the app so it owns
// the terminal: input, signals, and exit status behave as if you ran it directly.
func dev(ctx context.Context, dir string, args []string) error {
	p, err := detect(dir)
	if err != nil {
		return err
	}
	if err := p.buildWeb(ctx); err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(dir))
	bin := filepath.Join(cache, "go-kit", "dev", fmt.Sprintf("%x", sum[:8]), p.name)
	if err := run(ctx, dir, "go", "build", "-o", bin, "."); err != nil {
		return err
	}
	return syscall.Exec(bin, append([]string{bin}, args...), os.Environ())
}
