package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmeiracorbal/schema-guard/internal/cli"
)

// version is set by the release image build.
var version = "dev"

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, os.Getenv, version)
}
