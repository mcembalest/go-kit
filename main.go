package main

import (
	"context"
	"fmt"
	"github.com/mcembalest/go-kit/internal/cli"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "go-kit:", err)
		os.Exit(1)
	}
}
