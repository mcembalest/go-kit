package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func project(t *testing.T, updates bool) string {
	t.Helper()
	dir := t.TempDir()
	args := []string{"--module", "github.com/example/fixture", "--yes"}
	if updates {
		args = append(args, "--updates")
	}
	args = append(args, dir)
	if err := initProject(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	return dir
}
func inDir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}
func gitCommit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"git", "init", "-b", "main"}, {"git", "config", "user.name", "Fixture"}, {"git", "config", "user.email", "fixture@example.com"}, {"git", "add", "."}, {"git", "commit", "-m", "fixture"}} {
		if _, err := output(context.Background(), dir, args...); err != nil {
			t.Fatal(err)
		}
	}
}
func TestInitBuildAndVersion(t *testing.T) {
	dir := project(t, true)
	ctx := context.Background()
	if err := run(ctx, dir, "go", "test", "./..."); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "fixture")
	if err := run(ctx, dir, "go", "build", "-o", binary, "."); err != nil {
		t.Fatal(err)
	}
	out, err := output(ctx, dir, binary, "--version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fixture dev") || !strings.Contains(out, "commit=unknown") {
		t.Fatalf("unexpected identity: %s", out)
	}
	out, err = output(ctx, dir, binary)
	if err != nil || !strings.Contains(out, "fixture is ready") {
		t.Fatalf("%s: %v", out, err)
	}
	if err := initProject(ctx, []string{"--yes", dir}); err == nil {
		t.Fatal("reinitialization should not overwrite")
	}
}
func TestAdoptAndRejectConflict(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/example/fixture\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	source := []byte("package main\n// Keep this comment.\nfunc main(){ println(\"existing\") }\n")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), source, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gokit_build.go"), []byte("mine"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := initProject(context.Background(), []string{"--yes", dir}); err == nil {
		t.Fatal("expected conflict")
	}
	got, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	if string(got) != string(source) {
		t.Fatal("conflict changed main.go")
	}
	os.Remove(filepath.Join(dir, "gokit_build.go"))
	if err := initProject(context.Background(), []string{"--yes", dir}); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(filepath.Join(dir, "main.go"))
	if !strings.Contains(string(got), "existing") || !strings.Contains(string(got), "Keep this comment") || !strings.Contains(string(got), "gokitStartup()") {
		t.Fatalf("lost app behavior: %s", got)
	}
}
func TestSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	if err := os.Symlink(other, filepath.Join(dir, ".github")); err != nil {
		t.Fatal(err)
	}
	if err := initProject(context.Background(), []string{"--module", "github.com/example/fixture", "--yes", dir}); err == nil {
		t.Fatal("expected symlink rejection")
	}
	if exists(filepath.Join(other, "workflows", "gokit.yml")) {
		t.Fatal("wrote outside project")
	}
}
func TestShipDirtyHasNoSideEffects(t *testing.T) {
	dir := project(t, false)
	gitCommit(t, dir)
	inDir(t, dir)
	os.WriteFile("uncommitted", []byte("work"), 0644)
	if err := ship(context.Background(), []string{"v0.1.0", "--yes"}); err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("unexpected error: %v", err)
	}
	tags, err := output(context.Background(), dir, "git", "tag")
	if err != nil || tags != "" {
		t.Fatal("created tag before validating")
	}
}
func TestShipPreview(t *testing.T) {
	if os.Getenv("GOKIT_RELEASE_TEST") != "1" {
		t.Skip("set GOKIT_RELEASE_TEST=1 with GoReleaser installed")
	}
	dir := project(t, false)
	gitCommit(t, dir)
	inDir(t, dir)
	if _, err := output(context.Background(), dir, "git", "remote", "set-url", "origin", "https://github.com/example/fixture.git"); err != nil {
		t.Fatal(err)
	}
	if err := ship(context.Background(), []string{"v0.1.0", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	tags, _ := output(context.Background(), dir, "git", "tag")
	if tags != "" {
		t.Fatal("dry run created a tag")
	}
	if !exists(filepath.Join(dir, "dist", "checksums.txt")) {
		t.Fatal("no snapshot checksums")
	}
	if err := clean(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
}
func TestWeb(t *testing.T) {
	if os.Getenv("GOKIT_WEB_TEST") != "1" {
		t.Skip("set GOKIT_WEB_TEST=1 to run npm scaffold integration")
	}
	dir := t.TempDir()
	if err := initProject(context.Background(), []string{"--module", "github.com/example/browser", "--web", "--yes", dir}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dir, "web", "dist", "app.js")) || !exists(filepath.Join(dir, "web", "package-lock.json")) {
		t.Fatal("missing installation assets")
	}
	cleanInstall(t, dir, "github.com/example/browser")
	if exists(filepath.Join(dir, "web", "node_modules")) {
		t.Fatal("starter leaked staging dependencies")
	}
	if err := run(context.Background(), dir, "go", "build", "-o", filepath.Join(t.TempDir(), "browser"), "."); err != nil {
		t.Fatal(err)
	}
}
func TestFingerprint(t *testing.T) {
	dir := project(t, false)
	before, err := fingerprint(dir, Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "dist"), 0755)
	os.WriteFile(filepath.Join(dir, "dist", "artifact.go"), []byte("output"), 0644)
	after, _ := fingerprint(dir, Config{}, nil)
	if before != after {
		t.Fatal("release artifacts trigger rebuild")
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("changed"), 0644)
	after, _ = fingerprint(dir, Config{}, nil)
	if before == after {
		t.Fatal("source changes not watched")
	}
}
func TestPython(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not installed")
	}
	dir := t.TempDir()
	if err := initProject(context.Background(), []string{"--module", "github.com/example/pyapp", "--python", "--yes", dir}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"gokit_python.go", "python/worker.py", "python/pyproject.toml", "python/uv.lock"} {
		if !exists(filepath.Join(dir, p)) {
			t.Fatalf("missing %s", p)
		}
	}
	c, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Watch) != 1 || c.Watch[0] != "python" || len(c.Checks) != 1 || c.Checks[0][0] != "uv" {
		t.Fatalf("unexpected config %+v", c)
	}
	// the generated test starts the worker through uv and round-trips a call
	if err := run(context.Background(), dir, "go", "test", "-run", "TestGokitPython", "-v", "./..."); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), dir, c.Checks[0]...); err != nil {
		t.Fatal(err)
	}
}
