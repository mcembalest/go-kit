package cli

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed assets/*
var assets embed.FS

func initProject(ctx context.Context, args []string) error {
	fs := flags("init")
	module := fs.String("module", "", "GitHub module path; inferred from existing go.mod")
	web := fs.Bool("web", false, "include a TypeScript browser starter (new projects)")
	updates := fs.Bool("updates", false, "include opt-in app updates; off until enabled by its user")
	yes := fs.Bool("yes", false, "apply the printed plan without prompting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: go-kit init [flags] [directory]")
	}
	dir := "."
	if fs.NArg() == 1 {
		dir = fs.Arg(0)
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	dir, err = canonicalRoot(dir)
	if err != nil {
		return err
	}
	if exists(filepath.Join(dir, configName)) {
		return errors.New("already initialized; edit gokit.json to change this project")
	}
	adopting := exists(filepath.Join(dir, "go.mod"))
	if adopting {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "module" {
				found := strings.Trim(fields[1], `"`)
				if *module != "" && *module != found {
					return errors.New("--module differs from go.mod")
				}
				*module = found
				break
			}
		}
		if *web {
			return errors.New("--web scaffolds new apps; for an existing UI, set web in gokit.json after adoption")
		}
	}
	if !modulePattern.MatchString(*module) {
		return errors.New("provide --module github.com/owner/repo; initial support is a root Go executable")
	}
	name := filepath.Base(*module)
	files := map[string][]byte{}
	c := Config{Module: *module, Dev: DevConfig{Restart: "manual"}}
	if adopting {
		main, err := os.ReadFile(filepath.Join(dir, "main.go"))
		if err != nil {
			return errors.New("adoption requires root main.go; move the app entry point there first")
		}
		changed, err := wireStartup(main)
		if err != nil {
			return err
		}
		files["main.go"] = changed
	} else {
		files["go.mod"] = []byte("module " + *module + "\n\ngo 1.22\n")
		main := `package main
import "fmt"
func main() { gokitStartup(); fmt.Println("` + name + ` is ready.") }
`
		if *web {
			main = webMain
			c.Web = "web"
			c.Dev.Restart = "auto"
		}
		files["main.go"] = []byte(main)
	}
	for path, data := range map[string]string{
		"gokit_build.go":              strings.ReplaceAll(buildSource(name, *updates), "github.com/OWNER/REPO", *module),
		".goreleaser.yaml":            releaseConfig(name),
		".github/workflows/gokit.yml": testWorkflow(c),
	} {
		files[path] = []byte(data)
	}
	if *updates {
		for _, file := range []string{"autoupdate.go", "autoupdate_test.go"} {
			data, err := assets.ReadFile("assets/" + file + ".txt")
			if err != nil {
				return err
			}
			s := string(data)
			for _, n := range []string{"autoUpdate", "installUpdate", "stableVersion", "newerStable"} {
				s = strings.ReplaceAll(s, n, "gokit"+strings.ToUpper(n[:1])+n[1:])
			}
			files["gokit_"+file] = []byte(s)
		}
	}
	if *web {
		files["web/package.json"] = []byte(webPackage)
		files["web/tsconfig.json"] = []byte(`{"compilerOptions":{"target":"ES2022","module":"ESNext","lib":["ES2022","DOM"],"strict":true,"noEmit":true},"include":["src"]}` + "\n")
		files["web/src/app.ts"] = []byte("const app = document.querySelector<HTMLDivElement>('#app')!;\napp.textContent = '" + name + " is ready.';\n")
		files["web/index.html"] = []byte(webHTML)
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	files[configName] = append(b, '\n')
	if !exists(filepath.Join(dir, "README.md")) {
		files["README.md"] = []byte("# " + name + "\n\n```sh\ngo install " + *module + "@main\n" + name + "\n```\n\nDevelopment: `go-kit dev` · Release: `go-kit ship <version>`\n\n`" + name + " --version` identifies the installed build.\n")
	}
	if !exists(filepath.Join(dir, "DEPENDENCIES.md")) {
		files["DEPENDENCIES.md"] = []byte("# Dependencies\n\nInstall: Go 1.22+.\nDevelopment: Go" + map[bool]string{true: " and Node 24/npm", false: ""}[c.Web != ""] + ".\n\nReview before shipping: document external runtime programs, authentication,\nbundled assets/licenses, and separate example environments here.\n")
	}
	ignore, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	text := string(ignore)
	for _, line := range []string{"/dist/", "/bin/", "node_modules/"} {
		if !strings.Contains("\n"+text+"\n", "\n"+line+"\n") {
			text = strings.TrimRight(text, "\n") + "\n" + line + "\n"
		}
	}
	files[".gitignore"] = []byte(strings.TrimLeft(text, "\n"))
	paths := make([]string, 0, len(files))
	for p := range files {
		if err := safeDestination(dir, p); err != nil {
			return err
		}
		if exists(filepath.Join(dir, p)) && p != ".gitignore" && !(adopting && p == "main.go") {
			return fmt.Errorf("refusing to overwrite %s", p)
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	newGit := !exists(filepath.Join(dir, ".git"))
	step("Initialize " + dir)
	for _, p := range paths {
		verb := "create"
		if exists(filepath.Join(dir, p)) {
			verb = "update"
		}
		fmt.Printf("  %-7s %s\n", verb, p)
	}
	if *web {
		fmt.Println("  build   TypeScript starter (npm; requires network on first use)")
	}
	if newGit {
		fmt.Println("  create  Git repository and origin (no commit or push)")
	}
	if err := confirm(*yes, "Apply this project setup?"); err != nil {
		return err
	}
	if *web {
		stage, err := os.MkdirTemp("", "go-kit-web-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(stage)
		for p, data := range files {
			if strings.HasPrefix(p, "web/") {
				target := filepath.Join(stage, p)
				if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return err
				}
				if err := os.WriteFile(target, data, 0644); err != nil {
					return err
				}
			}
		}
		for _, args := range [][]string{{"npm", "--prefix", "web", "install", "--package-lock-only", "--ignore-scripts"}, {"npm", "--prefix", "web", "ci"}, {"npm", "--prefix", "web", "run", "build"}} {
			if err := run(ctx, stage, args...); err != nil {
				return err
			}
		}
		for _, p := range []string{"web/package-lock.json", "web/dist/app.js", "web/dist/index.html"} {
			data, err := os.ReadFile(filepath.Join(stage, p))
			if err != nil {
				return err
			}
			files[p] = data
			paths = append(paths, p)
		}
	}
	// Validate every destination again before applying, including generated UI files.
	for _, p := range paths {
		if err := safeDestination(dir, p); err != nil {
			return err
		}
	}
	for _, p := range paths {
		data := files[p]
		if strings.HasSuffix(p, ".go") {
			data, err = format.Source(data)
			if err != nil {
				return fmt.Errorf("format %s: %w", p, err)
			}
		}
		target := filepath.Join(dir, p)
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(target, data, 0644); err != nil {
			return err
		}
	}
	if newGit {
		if err := run(ctx, dir, "git", "init", "-b", "main"); err != nil {
			return err
		}
		if err := run(ctx, dir, "git", "remote", "add", "origin", "https://"+*module+".git"); err != nil {
			return err
		}
	}
	fmt.Println("\nReady. Run go-kit dev in", dir)
	if adopting {
		fmt.Println("Review gokit.json build, watch, checks, and the generated CI targets before shipping.")
	}
	return nil
}

func safeDestination(root, path string) error {
	target := filepath.Join(root, path)
	for p := target; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink destination %s", p)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	return nil
}
func wireStartup(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if f.Name.Name != "main" {
		return nil, errors.New("root main.go must declare package main")
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "main" && fn.Recv == nil && fn.Body != nil {
			fn.Body.List = append([]ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("gokitStartup")}}}, fn.Body.List...)
			var out bytes.Buffer
			err = format.Node(&out, fset, f)
			return out.Bytes(), err
		}
	}
	return nil, errors.New("no main function found in main.go")
}

func canonicalRoot(path string) (string, error) {
	var tail []string
	for !exists(path) {
		tail = append(tail, filepath.Base(path))
		parent := filepath.Dir(path)
		if parent == path {
			return "", errors.New("cannot resolve project directory")
		}
		path = parent
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	for i := len(tail) - 1; i >= 0; i-- {
		resolved = filepath.Join(resolved, tail[i])
	}
	return resolved, nil
}
