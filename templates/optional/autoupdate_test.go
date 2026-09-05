package main

import (
	"archive/zip"
	"context"
	"debug/buildinfo"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestStableUpdates(t *testing.T) {
	for _, tc := range []struct {
		next, current string
		want          bool
	}{
		{"v0.2.0", "v0.1.0", true}, {"v0.10.0", "v0.9.0", true},
		{"v0.1.0", "v0.1.0", false}, {"v0.1.0", "v0.2.0", false},
		{"v0.2.0-alpha.1", "v0.1.0", false}, {"v0.2.0", "dev", false},
		{"v0.2.0", "v0.1.0-alpha.1", false}, {"v01.2.0", "v0.1.0", false},
	} {
		if got := newerStable(tc.next, tc.current); got != tc.want {
			t.Errorf("%s -> %s: got %v", tc.current, tc.next, got)
		}
	}
}

// Exercise real Go module installation through a local file proxy: no network,
// published releases, product binaries, or user installation directories.
func TestInstallUpdate(t *testing.T) {
	dir := t.TempDir()
	proxy := filepath.Join(dir, "proxy", "example.com", "fixture", "@v")
	if err := os.MkdirAll(proxy, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	mod := []byte("module example.com/fixture\n\ngo 1.22\n")
	for _, v := range []string{"v0.1.0", "v0.2.0"} {
		write(filepath.Join(proxy, v+".mod"), mod)
		write(filepath.Join(proxy, v+".info"), []byte(fmt.Sprintf(`{"Version":%q,"Time":"2026-09-05T00:00:00Z"}`, v)))
		f, err := os.Create(filepath.Join(proxy, v+".zip"))
		if err != nil {
			t.Fatal(err)
		}
		z := zip.NewWriter(f)
		for name, data := range map[string][]byte{"go.mod": mod, "main.go": []byte("package main\nfunc main() {}\n")} {
			w, err := z.Create("example.com/fixture@" + v + "/" + name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write(data); err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(proxy, "list"), []byte("v0.1.0\nv0.2.0\n"))
	bin := filepath.Join(dir, "bin with spaces")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPROXY", "file://"+filepath.Join(dir, "proxy"))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOMODCACHE", filepath.Join(dir, "cache"))
	t.Setenv("GOBIN", bin)
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "-modcacherw")
	cmd := exec.Command("go", "install", "example.com/fixture@v0.1.0")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	path := filepath.Join(bin, "fixture")
	assertVersion := func(want string) {
		t.Helper()
		info, err := buildinfo.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Main.Version != want {
			t.Fatalf("got %s, want %s", info.Main.Version, want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := installUpdate(ctx, "example.com/fixture", "v0.1.0", path); err == nil {
		t.Fatal("expected cancelled query")
	}
	assertVersion("v0.1.0")
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := installUpdate(ctx, "example.com/fixture", "v0.1.0", path); err != nil {
		t.Fatal(err)
	}
	assertVersion("v0.2.0")
	// A process still running the old build must leave the updated installation alone.
	if err := installUpdate(ctx, "example.com/fixture", "v0.1.0", path); err != nil {
		t.Fatal(err)
	}
	assertVersion("v0.2.0")
	// A malformed new module must never replace the working executable.
	write(filepath.Join(proxy, "v0.3.0.mod"), mod)
	write(filepath.Join(proxy, "v0.3.0.info"), []byte(`{"Version":"v0.3.0","Time":"2026-09-05T00:00:00Z"}`))
	write(filepath.Join(proxy, "v0.3.0.zip"), []byte("broken archive"))
	write(filepath.Join(proxy, "list"), []byte("v0.1.0\nv0.2.0\nv0.3.0\n"))
	if err := installUpdate(ctx, "example.com/fixture", "v0.2.0", path); err == nil {
		t.Fatal("expected broken download")
	}
	assertVersion("v0.2.0")
	// Unavailable proxy must also preserve the working executable.
	t.Setenv("GOPROXY", "file://"+filepath.Join(dir, "missing"))
	if err := installUpdate(ctx, "example.com/fixture", "v0.2.0", path); err == nil {
		t.Fatal("expected unavailable proxy")
	}
	assertVersion("v0.2.0")
}

func TestAutoUpdateOff(t *testing.T) {
	t.Setenv("FIXTURE_AUTO_UPDATE", "0")
	t.Setenv("PATH", "")
	autoUpdate("example.com/fixture", "FIXTURE_AUTO_UPDATE")
}
