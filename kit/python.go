package kit

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sync"
)

// Worker is a Python process speaking JSON lines: one request per stdin line,
// one response per stdout line. Its stderr passes through to the app's stderr.
type Worker struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Scanner
	mu  sync.Mutex
}

// Python starts main.py from a uv project embedded in the app, e.g.
//
//	//go:embed python
//	var python embed.FS
//	w, err := kit.Python(python, "myapp")
//
// The project (pyproject.toml, uv.lock, main.py, other .py files) is copied to
// <user cache>/<app>/python and run with `uv run --locked`, so Python and packages
// install on first use and stay in one reusable environment. If uv isn't installed,
// a pinned uv is downloaded and verified first.
func Python(files fs.FS, app string) (*Worker, error) {
	if _, err := fs.Stat(files, "python/pyproject.toml"); err == nil {
		files, _ = fs.Sub(files, "python")
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, app, "python")
	if err := copyProject(files, dir); err != nil {
		return nil, err
	}
	uv, err := uvPath()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(uv, "run", "--locked", "--quiet", "--project", dir, "python", filepath.Join(dir, "main.py"))
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	return &Worker{cmd: cmd, in: in, out: sc}, nil
}

// Call sends req and decodes the response into resp (which may be nil).
// A response with an "error" field fails the call.
func (w *Worker) Call(req, resp any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if _, err := w.in.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("python: %w", err)
	}
	if !w.out.Scan() {
		if err := w.out.Err(); err != nil {
			return fmt.Errorf("python: %w", err)
		}
		return errors.New("python exited")
	}
	var failed struct {
		Error *string `json:"error"`
	}
	if json.Unmarshal(w.out.Bytes(), &failed) == nil && failed.Error != nil {
		return fmt.Errorf("python: %s", *failed.Error)
	}
	if resp == nil {
		return nil
	}
	return json.Unmarshal(w.out.Bytes(), resp)
}

// Close ends the worker by closing its input and waits for it to exit.
func (w *Worker) Close() error {
	w.in.Close()
	return w.cmd.Wait()
}

// copyProject mirrors the embedded project into dir, writing only files that changed.
func copyProject(files fs.FS, dir string) error {
	if _, err := fs.Stat(files, "main.py"); err != nil {
		return errors.New("embedded python project needs main.py")
	}
	return fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(files, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(p))
		if old, err := os.ReadFile(target); err == nil && bytes.Equal(old, data) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		tmp := target + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, target)
	})
}

// Pinned uv used when uv isn't on PATH.
const uvVersion = "0.12.19"

var uvSHA256 = map[string]string{
	"aarch64-apple-darwin":      "a9a8df1eedeb192f2e47e40e2faabfb387db4b850209118786d42f89dde3e0ba",
	"x86_64-apple-darwin":       "cb5fa57bafe68fc0fb94b17f06bee0b0b9a7feb94ccbd110445afa0696e39273",
	"aarch64-unknown-linux-gnu": "0804e9b164c64b6914182d5920c08551958a095986f10a3731056df701126436",
	"x86_64-unknown-linux-gnu":  "23bf5552d220e0842b65c862097b2ebaeba0064b74eda5e565e77fd25969d8c8",
}

func uvPath() (string, error) {
	if p, err := exec.LookPath("uv"); err == nil {
		return p, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	bin := filepath.Join(base, "go-kit", "uv-"+uvVersion, "uv")
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	return bin, downloadUV(bin)
}

func downloadUV(bin string) error {
	arch := map[string]string{"arm64": "aarch64", "amd64": "x86_64"}[runtime.GOARCH]
	osName := map[string]string{"darwin": "apple-darwin", "linux": "unknown-linux-gnu"}[runtime.GOOS]
	triple := arch + "-" + osName
	sum, ok := uvSHA256[triple]
	if arch == "" || osName == "" || !ok {
		return fmt.Errorf("no uv build for %s/%s; install uv: https://docs.astral.sh/uv/", runtime.GOOS, runtime.GOARCH)
	}
	fmt.Fprintf(os.Stderr, "downloading uv %s (one time)\n", uvVersion)
	resp, err := http.Get(fmt.Sprintf("https://github.com/astral-sh/uv/releases/download/%s/uv-%s.tar.gz", uvVersion, triple))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("downloading uv: %s", resp.Status)
	}
	archive, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if got := sha256.Sum256(archive); hex.EncodeToString(got[:]) != sum {
		return errors.New("downloaded uv failed its checksum")
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err != nil {
			return errors.New("uv binary not found in archive")
		}
		if path.Base(h.Name) != "uv" || h.Typeflag != tar.TypeReg {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
			return err
		}
		tmp := bin + ".tmp"
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, tr)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		return os.Rename(tmp, bin)
	}
}
