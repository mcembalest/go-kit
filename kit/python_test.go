package kit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPython(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not installed")
	}
	if base, err := os.UserCacheDir(); err == nil {
		t.Cleanup(func() { os.RemoveAll(filepath.Join(base, "kit-test")) })
	}
	w, err := Python(os.DirFS("testdata"), "kit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	var resp struct{ Echo map[string]string }
	if err := w.Call(map[string]string{"hi": "there"}, &resp); err != nil || resp.Echo["hi"] != "there" {
		t.Fatalf("%+v %v", resp, err)
	}
	if err := w.Call(map[string]string{"fail": "on purpose"}, nil); err == nil || err.Error() != "python: on purpose" {
		t.Fatalf("expected worker error, got %v", err)
	}
}

func TestCopyProjectOnlyRewritesChanges(t *testing.T) {
	dir := t.TempDir()
	if err := copyProject(os.DirFS("testdata/python"), dir); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "main.py")
	st, _ := os.Stat(f)
	if err := copyProject(os.DirFS("testdata/python"), dir); err != nil {
		t.Fatal(err)
	}
	if st2, _ := os.Stat(f); !st2.ModTime().Equal(st.ModTime()) {
		t.Fatal("unchanged file was rewritten")
	}
}

func TestDownloadUV(t *testing.T) {
	if os.Getenv("GOKIT_NET_TEST") != "1" {
		t.Skip("set GOKIT_NET_TEST=1 to download uv")
	}
	bin := filepath.Join(t.TempDir(), "uv")
	if err := downloadUV(bin); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "--version").Output(); err != nil || len(out) == 0 {
		t.Fatalf("%s %v", out, err)
	}
}
