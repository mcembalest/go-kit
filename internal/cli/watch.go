package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Go resolves embed patterns; no TypeScript/frontend assumption is involved.
func embeddedFiles(ctx context.Context, dir string) ([]string, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	dir = resolved
	cmd := command(ctx, dir, "go", "list", "-e", "-json", "./...")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("discover embedded files: %w", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	var files []string
	for {
		var pkg struct {
			Dir           string
			EmbedPatterns []string
		}
		if err := dec.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		for _, p := range pkg.EmbedPatterns {
			p = strings.TrimPrefix(p, "all:")
			if i := strings.IndexAny(p, "*?["); i >= 0 {
				p = filepath.Dir(p[:i] + "pattern")
			}
			rel, err := filepath.Rel(dir, filepath.Join(pkg.Dir, p))
			if err == nil && relativePath(rel) {
				files = append(files, rel)
			}
		}
	}
	return files, nil
}
func fingerprint(dir string, c Config, embeds []string) ([32]byte, error) {
	h := sha256.New()
	embedded := map[string]bool{}
	for _, p := range embeds {
		embedded[p] = true
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		explicit := false
		ancestor := false
		for _, p := range c.Watch {
			if p == "." || rel == p || strings.HasPrefix(rel, p+string(filepath.Separator)) {
				explicit = true
			}
			if strings.HasPrefix(p, rel+string(filepath.Separator)) {
				ancestor = true
			}
		}
		if d.IsDir() {
			// Git and dependency trees are never source watches. Other ignored paths
			// can be opted into explicitly (e.g. externally produced embedded assets).
			if rel != "." && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if rel != "." && !explicit && !ancestor && (strings.HasPrefix(d.Name(), ".") || d.Name() == "bin" || d.Name() == "dist") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !(explicit || embeddedPath(embedded, rel) || strings.HasSuffix(rel, ".go") || rel == "go.mod" || rel == "go.sum" || rel == configName || (c.Web != "" && strings.HasPrefix(rel, c.Web+string(filepath.Separator)))) {
			return nil
		}
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(h, rel)
		h.Write(data)
		return nil
	})
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, err
}

func embeddedPath(roots map[string]bool, path string) bool {
	for p := range roots {
		if p == "." || path == p || strings.HasPrefix(path, p+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
