.DEFAULT_GOAL := build
APP := myapp
.PHONY: run check build release-check

run:
	go run . $(ARGS)

check:
	go test -race ./...
	go vet ./...

build:
	go build -o bin/$(APP) .

release-check: check
	goreleaser check
	goreleaser release --snapshot --clean
