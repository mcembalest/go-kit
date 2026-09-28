package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixture writes a tiny app that prints kit.Version() and its arguments, committed in a fresh git repo.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	here := moduleRoot(t)
	files := map[string]string{
		"go.mod": "module github.com/example/demo\n\ngo 1.22\n\nrequire github.com/mcembalest/go-kit v0.0.0\n\nreplace github.com/mcembalest/go-kit => " + here + "\n",
		"main.go": `package main

import (
	"fmt"
	"os"

	"github.com/mcembalest/go-kit/kit"
)

func main() { fmt.Println("demo", kit.Version(), os.Args[1:]) }
`,
		"README.md": "# demo\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"git", "init", "-q", "-b", "main"}, {"git", "add", "."},
		{"git", "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "demo"}} {
		if out, err := output(context.Background(), dir, args...); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	return dir
}

func TestDetect(t *testing.T) {
	p, err := detect(fixture(t))
	if err != nil || p.module != "github.com/example/demo" || p.name != "demo" || p.web || p.python {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := detect(t.TempDir()); err == nil {
		t.Fatal("expected an error without go.mod")
	}
}

func TestShipDryRunAndInstallScript(t *testing.T) {
	dir := fixture(t)
	out, err := ship(context.Background(), dir, "v1.2.3", true, false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(out)
	for _, tg := range targets {
		if !exists(filepath.Join(out, "demo_v1.2.3_"+tg[0]+"_"+tg[1]+".tar.gz")) {
			t.Fatalf("missing archive for %v", tg)
		}
	}
	sums, _ := os.ReadFile(filepath.Join(out, "checksums.txt"))
	if strings.Count(string(sums), "\n") != len(targets) {
		t.Fatalf("checksums: %s", sums)
	}
	// run install.sh against the local artifacts instead of GitHub
	script, _ := os.ReadFile(filepath.Join(out, "install.sh"))
	local := strings.Replace(string(script), "url=https://github.com/example/demo/releases/download/$version", "url=file://"+out, 1)
	if local == string(script) {
		t.Fatal("install.sh download URL not found")
	}
	bin := t.TempDir()
	cmd := exec.Command("sh", "-c", local)
	cmd.Env = append(os.Environ(), "INSTALL_DIR="+bin)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install.sh: %v\n%s", err, b)
	}
	got, err := exec.Command(filepath.Join(bin, "demo"), "x").Output()
	if err != nil || strings.TrimSpace(string(got)) != "demo v1.2.3 [x]" {
		t.Fatalf("installed binary printed %q (%v); on %s", got, err, runtime.GOOS)
	}
	if _, err := ship(context.Background(), dir, "1.2", true, false); err == nil {
		t.Fatal("expected a version format error")
	}
}

func TestShipRefusesDirtyTree(t *testing.T) {
	dir := fixture(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644)
	if _, err := ship(context.Background(), dir, "v0.1.0", false, true); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("expected dirty-tree refusal, got %v", err)
	}
}

func TestDev(t *testing.T) {
	gokit := filepath.Join(t.TempDir(), "go-kit")
	if out, err := output(context.Background(), moduleRoot(t), "go", "build", "-o", gokit, "."); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	out, err := output(context.Background(), fixture(t), gokit, "dev", "--", "a", "b")
	if err != nil || !strings.Contains(out, "demo ") || !strings.HasSuffix(out, " [a b]") {
		t.Fatalf("%q %v", out, err)
	}
}

// moduleRoot is the go-kit repo root (tests run in internal/cli).
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for !exists(filepath.Join(dir, "go.mod")) {
		if dir == filepath.Dir(dir) {
			t.Fatal("go.mod not found")
		}
		dir = filepath.Dir(dir)
	}
	return dir
}
