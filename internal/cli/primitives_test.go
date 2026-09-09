package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func setConfig(t *testing.T, dir string, c Config) {
	t.Helper()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, configName), data, 0644); err != nil {
		t.Fatal(err)
	}
}
func TestGenericInputs(t *testing.T) {
	dir := project(t, false)
	source := `package main
import _ "embed"
//go:embed assets/*.ts
var extension string
func main(){}
`
	os.Mkdir(filepath.Join(dir, "assets"), 0755)
	os.WriteFile(filepath.Join(dir, "assets", "extension.ts"), []byte("first"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0644)
	embeds, err := embeddedFiles(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := fingerprint(dir, Config{}, embeds)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "assets", "extension.ts"), []byte("second"), 0644)
	after, _ := fingerprint(dir, Config{}, embeds)
	if after == before {
		t.Fatal("embedded TS was not watched")
	}
	before = after
	os.WriteFile(filepath.Join(dir, "assets", "another.ts"), []byte("new"), 0644)
	after, _ = fingerprint(dir, Config{}, embeds)
	if after == before {
		t.Fatal("new glob match was not watched")
	}
	c := Config{Watch: []string{"scripts", ".inputs"}}
	os.Mkdir(filepath.Join(dir, "scripts"), 0755)
	os.Mkdir(filepath.Join(dir, ".inputs"), 0755)
	before, _ = fingerprint(dir, c, embeds)
	os.WriteFile(filepath.Join(dir, "scripts", "task.py"), []byte("print('hello')"), 0644)
	after, _ = fingerprint(dir, c, embeds)
	if after == before {
		t.Fatal("optional-language input not watched")
	}
	before = after
	os.WriteFile(filepath.Join(dir, ".inputs", "data"), []byte("input"), 0644)
	after, _ = fingerprint(dir, c, embeds)
	if after == before {
		t.Fatal("explicit hidden input not watched")
	}
}
func TestGenericBuild(t *testing.T) {
	dir := project(t, false)
	script := filepath.Join(dir, "build.go")
	os.WriteFile(script, []byte(`//go:build ignore
package main
import("os";"strings")
func main(){os.WriteFile("result",[]byte(strings.Join(os.Args[1:],"|")),0644)}
`), 0644)
	c := Config{Web: "does-not-exist", Build: [][]string{{"go", "run", "build.go", "with spaces", "literal $value"}}}
	if err := buildAssets(context.Background(), dir, c); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "result"))
	if string(data) != "with spaces|literal $value" {
		t.Fatalf("arguments changed: %q", data)
	}
	c.Build = [][]string{}
	if err := buildAssets(context.Background(), dir, c); err != nil {
		t.Fatal("explicit empty builds should disable web shorthand", err)
	}
}
func TestConfigPrimitives(t *testing.T) {
	dir := project(t, false)
	for _, c := range []Config{
		{Dev: DevConfig{Restart: "sometimes"}}, {Watch: []string{"../outside"}}, {Watch: []string{"*.ts"}}, {Build: [][]string{{}}},
	} {
		c.Module = "github.com/example/fixture"
		setConfig(t, dir, c)
		if _, err := load(dir); err == nil {
			t.Fatalf("accepted invalid config %#v", c)
		}
	}
	setConfig(t, dir, Config{Module: "github.com/example/fixture"})
	c, err := load(dir)
	if err != nil || c.Dev.Restart != "manual" {
		t.Fatalf("unsafe default %#v: %v", c, err)
	}
}

// A separate test process is deliberately replaced by the app, just like the CLI.
func TestManualDevHelper(t *testing.T) {
	if os.Getenv("GOKIT_MANUAL_HELPER") != "1" {
		return
	}
	os.Setenv("EXPECTED_PID", fmt.Sprint(os.Getpid()))
	args := []string(nil)
	if os.Getenv("OVERRIDE_ARGS") == "1" {
		args = []string{"--", "override with spaces"}
	}
	if err := dev(context.Background(), args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(99)
	}
	os.Exit(98)
}
func TestManualDev(t *testing.T) {
	dir := project(t, false)
	cleanupDevCache(t, dir)
	setConfig(t, dir, Config{Module: "github.com/example/fixture", Dev: DevConfig{Args: []string{"saved with spaces"}, Restart: "manual"}})
	source := `package main
import("bufio";"fmt";"os";"strings")
func main(){
 if fmt.Sprint(os.Getpid())!=os.Getenv("EXPECTED_PID"){panic("process was wrapped")}
 line,_:=bufio.NewReader(os.Stdin).ReadString('\n')
 fmt.Printf("APP:%s:%s\n",strings.Join(os.Args[1:],"|"),strings.TrimSpace(line))
 os.Exit(17)
}`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0644)
	for _, override := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestManualDevHelper$")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOKIT_MANUAL_HELPER=1", "OVERRIDE_ARGS="+map[bool]string{true: "1", false: "0"}[override])
		cmd.Stdin = strings.NewReader("keyboard input\n")
		out, err := cmd.CombinedOutput()
		cancel()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 17 {
			t.Fatalf("exit status lost: %s: %v", out, err)
		}
		want := "saved with spaces"
		if override {
			want = "override with spaces"
		}
		if !strings.Contains(string(out), "APP:"+want+":keyboard input") {
			t.Fatalf("input/arguments lost: %s", out)
		}
	}
}
func TestTerminalDev(t *testing.T) {
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("system script utility is needed for PTY validation")
	}
	dir := project(t, false)
	cleanupDevCache(t, dir)
	source := `package main
import("bufio";"fmt";"os";"os/exec";"strings";"os/signal";"syscall";"time")
func main(){
 cmd:=exec.Command("sh","-c","test -t 0");cmd.Stdin=os.Stdin;if err:=cmd.Run();err!=nil{panic("no terminal")}
 resized:=make(chan os.Signal,1);signal.Notify(resized,syscall.SIGWINCH)
 cmd=exec.Command("stty","rows","31","cols","93");cmd.Stdin=os.Stdin;if err:=cmd.Run();err!=nil{panic(err)}
 select{case <-resized:case <-time.After(time.Second):panic("resize signal missing")}
 cmd=exec.Command("stty","size");cmd.Stdin=os.Stdin;size,err:=cmd.Output();if err!=nil{panic(err)}
 interrupted:=make(chan os.Signal,1);signal.Notify(interrupted,os.Interrupt)
 fmt.Println("TTYREADY")
 line,_:=bufio.NewReader(os.Stdin).ReadString('\n')
 fmt.Printf("PTY:%s:%s\n",strings.TrimSpace(string(size)),strings.TrimSpace(line))
 fmt.Println("INTREADY"); <-interrupted; fmt.Println("INTOK")
}`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0644)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.CommandContext(ctx, script, "-q", "/dev/null", os.Args[0], "-test.run=^TestManualDevHelper$")
	} else {
		quoted := "'" + strings.ReplaceAll(os.Args[0], "'", "'\"'\"'") + "'"
		cmd = exec.CommandContext(ctx, script, "-q", "-e", "-c", quoted+" -test.run=^TestManualDevHelper$", "/dev/null")
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOKIT_MANUAL_HELPER=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	reader, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout
	ready := make(chan struct{}, 1)
	intReady := make(chan struct{}, 1)
	text := make(chan string, 1)
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		var lines strings.Builder
		s := bufio.NewScanner(reader)
		for s.Scan() {
			lines.WriteString(s.Text() + "\n")
			if strings.Contains(s.Text(), "INTREADY") {
				select {
				case intReady <- struct{}{}:
				default:
				}
			}
			if strings.Contains(s.Text(), "TTYREADY") {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
		}
		text <- lines.String()
	}()
	select {
	case <-ready:
		input.Write([]byte("terminal input\n"))
	case <-ctx.Done():
		input.Close()
	}
	select {
	case <-intReady:
		input.Write([]byte{3})
	case <-ctx.Done():
		input.Close()
	}
	out := <-text
	err = cmd.Wait()
	if err != nil || !strings.Contains(string(out), "PTY:31 93:terminal input") || !strings.Contains(string(out), "INTOK") {
		t.Fatalf("terminal failed: %v\n%s", err, out)
	}
}

func cleanupDevCache(t *testing.T, dir string) {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(canonical))
	path := filepath.Join(root, "go-kit", "dev", fmt.Sprintf("%x", sum[:16]))
	t.Cleanup(func() { os.RemoveAll(path) })
}
