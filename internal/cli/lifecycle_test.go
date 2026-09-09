package cli

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func cleanInstall(t *testing.T, dir, module string) {
	t.Helper()
	base := t.TempDir()
	proxy := filepath.Join(base, "proxy", module, "@v")
	if err := os.MkdirAll(proxy, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"v0.1.0.mod": data, "v0.1.0.info": []byte(`{"Version":"v0.1.0","Time":"2026-09-06T00:00:00Z"}`), "list": []byte("v0.1.0\n")} {
		if err := os.WriteFile(filepath.Join(proxy, name), body, 0644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Create(filepath.Join(proxy, "v0.1.0.zip"))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(module + "@v0.1.0/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Setenv("GOPROXY", "file://"+filepath.Join(base, "proxy"))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOMODCACHE", filepath.Join(base, "cache"))
	t.Setenv("GOFLAGS", "-modcacherw")
	t.Setenv("GOBIN", filepath.Join(base, "bin"))
	t.Setenv("GOWORK", "off")
	t.Setenv("CGO_ENABLED", "0")
	if err := run(context.Background(), base, "go", "install", module+"@v0.1.0"); err != nil {
		t.Fatal(err)
	}
	out, err := output(context.Background(), base, filepath.Join(base, "bin", filepath.Base(module)), "--version")
	if err != nil || !strings.Contains(out, "v0.1.0") {
		t.Fatalf("source installation identity: %s: %v", out, err)
	}
}
func TestCleanInstall(t *testing.T) { cleanInstall(t, project(t, false), "github.com/example/fixture") }

func TestDevRebuildAndStop(t *testing.T) {
	dir := project(t, false)
	setConfig(t, dir, Config{Module: "github.com/example/fixture", Dev: DevConfig{Restart: "auto"}})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	source := func(message string) []byte {
		return []byte(fmt.Sprintf("package main\nimport(\"net/http\";\"fmt\")\nfunc main(){gokitStartup(); http.HandleFunc(\"/\",func(w http.ResponseWriter,r *http.Request){fmt.Fprint(w,%q)});if err:=http.ListenAndServe(%q,nil);err!=nil{panic(err)}}\n", message, address))
	}
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, source("first"), 0644)
	inDir(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- dev(ctx, nil) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(10 * time.Second):
			t.Error("dev did not stop")
		}
	}()
	client := http.Client{Timeout: 200 * time.Millisecond}
	wait := func(want string) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			r, e := client.Get("http://" + address)
			if e == nil {
				b, _ := io.ReadAll(r.Body)
				r.Body.Close()
				if string(b) == want {
					return
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("never served %q", want)
	}
	wait("first")
	os.WriteFile(path, []byte("invalid go"), 0644)
	time.Sleep(time.Second)
	wait("first")
	os.WriteFile(path, source("second"), 0644)
	wait("second")
}
func executable(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0755); err != nil {
		t.Fatal(err)
	}
}
func TestShipPublicationBoundaries(t *testing.T) {
	dir := project(t, false)
	gitCommit(t, dir)
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	if _, err := output(context.Background(), base, "git", "init", "--bare", remote); err != nil {
		t.Fatal(err)
	}
	if _, err := output(context.Background(), dir, "git", "remote", "set-url", "origin", remote); err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(base, "tools")
	os.Mkdir(bin, 0755)
	t.Setenv("REAL_GIT", realGit)
	executable(t, filepath.Join(bin, "git"), `#!/bin/sh
if [ "$1" = remote ] && [ "$2" = get-url ]; then echo https://github.com/example/fixture.git; exit 0; fi
exec "$REAL_GIT" "$@"
`)
	executable(t, filepath.Join(bin, "goreleaser"), `#!/bin/sh
if [ "$1" = check ]; then exit 0; fi
if [ "$FAIL_PACKAGE" = 1 ]; then exit 8; fi
mkdir -p dist
printf fixture > dist/fixture.tar.gz
printf fixture > dist/checksums.txt
`)
	executable(t, filepath.Join(bin, "gh"), `#!/bin/sh
if [ "$1" = auth ]; then exit 0; fi
if [ "$FAIL_PUBLISH" = 1 ]; then exit 9; fi
printf '%s\n' "$*" >> "$PUBLISH_LOG"
`)
	t.Setenv("PUBLISH_LOG", filepath.Join(base, "publish.log"))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	inDir(t, dir)
	t.Setenv("FAIL_PACKAGE", "1")
	if err := ship(context.Background(), []string{"v0.1.0", "--yes"}); err == nil {
		t.Fatal("expected package failure")
	}
	tags, _ := output(context.Background(), dir, "git", "tag")
	if tags != "" {
		t.Fatal("failed package left a tag")
	}
	t.Setenv("FAIL_PACKAGE", "0")
	if err := ship(context.Background(), []string{"v0.1.0", "--yes"}); err != nil {
		t.Fatal(err)
	}
	tags, _ = output(context.Background(), base, "git", "--git-dir", remote, "tag")
	if tags != "v0.1.0" {
		t.Fatalf("remote tags: %s", tags)
	}
	log, err := os.ReadFile(filepath.Join(base, "publish.log"))
	if err != nil || !strings.Contains(string(log), "release create v0.1.0") {
		t.Fatal("release not requested")
	}
	t.Setenv("FAIL_PUBLISH", "1")
	if err := ship(context.Background(), []string{"v0.2.0", "--yes"}); err == nil || !strings.Contains(err.Error(), "tag is pushed") {
		t.Fatalf("unexpected error %v", err)
	}
	tags, _ = output(context.Background(), base, "git", "--git-dir", remote, "tag")
	if !strings.Contains(tags, "v0.2.0") {
		t.Fatal("post-push failure lost tag")
	}
}
