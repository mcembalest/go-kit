package cli

import "strings"

func buildSource(name string, updates bool) string {
	extra := ""
	if updates {
		extra = "gokitAutoUpdate(\"github.com/OWNER/REPO\", \"" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_AUTO_UPDATE\")"
	}
	return `package main
import("fmt"; "os"; "runtime"; "runtime/debug")
var gokitVersion, gokitCommit string
func gokitStartup(){
 if len(os.Args)>1 && (os.Args[1]=="--version" || os.Args[1]=="-v") {
  v,commit,dirty:=gokitVersion,gokitCommit,"unknown"
  if info,ok:=debug.ReadBuildInfo();ok{
   if v=="" && info.Main.Version!="(devel)"{v=info.Main.Version}
   for _,s:=range info.Settings{switch s.Key{case "vcs.revision":if commit==""{commit=s.Value};case "vcs.modified":dirty=s.Value}}
  }
  if v==""{v="dev"};if commit==""{commit="unknown"}
  fmt.Printf("` + name + ` %s (commit=%s dirty=%s %s %s/%s)\n",v,commit,dirty,runtime.Version(),runtime.GOOS,runtime.GOARCH)
  os.Exit(0)
 }
 ` + extra + `
}
`
}
func releaseConfig(name string) string {
	return `version: 2
project_name: ` + name + `
builds:
  - main: .
    binary: ` + name + `
    env: [CGO_ENABLED=0]
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w -X main.gokitVersion=v{{.Version}} -X main.gokitCommit={{.FullCommit}}
archives:
  - formats: [tar.gz]
    files: [README.md, DEPENDENCIES.md, LICENSE*]
checksum:
  name_template: checksums.txt
changelog:
  sort: asc
`
}
func testWorkflow(c Config, python bool) string {
	web := ""
	if python {
		web = `      - uses: astral-sh/setup-uv@v6
      - run: uv lock --check --project python
`
	}
	if c.Web != "" {
		web += `      - uses: actions/setup-node@v4
        with:
          node-version: '24'
      - run: npm --prefix ` + c.Web + ` ci
      - run: npm --prefix ` + c.Web + ` run build
      - run: git diff --exit-code HEAD
      - run: test -z "$(git ls-files --others --exclude-standard)"
`
	}
	return `name: Go project
on: [push, pull_request]
permissions:
  contents: read
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-15]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
` + web + `      - run: go test -race ./...
      - run: go vet ./...
      - run: go build ./...
`
}

const webPackage = `{
  "private": true,
  "scripts": {
    "build": "tsc --noEmit && esbuild src/app.ts --bundle --minify --outfile=dist/app.js && cp index.html dist/index.html"
  },
  "devDependencies": {"esbuild": "0.28.2", "typescript": "7.0.2"}
}
`
const webHTML = `<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Ready</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#111318;color:#e5e7eb;font:24px system-ui}#app{padding:3rem;border:1px solid #303641;border-radius:16px}</style>
<div id="app"></div><script src="/app.js"></script></html>
`
const webMain = `package main
import("embed";"io/fs";"log";"net/http";"os")
//go:embed web/dist
var web embed.FS
func main(){
 gokitStartup()
 address:=os.Getenv("ADDR");if address==""{address="127.0.0.1:8080"}
 root,err:=fs.Sub(web,"web/dist");if err!=nil{log.Fatal(err)}
 log.Printf("http://%s",address)
 log.Fatal(http.ListenAndServe(address,http.FileServer(http.FS(root))))
}
`

func pythonProject(name string) string {
	return `[project]
name = "` + name + `-python"
version = "0"
requires-python = ">=3.12"
dependencies = []

[tool.uv]
package = false
`
}
