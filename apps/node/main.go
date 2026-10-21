package main

import (
	"context"
	"os"

	"orion/node/internal/cli"
)

func main() {
	os.Exit(cli.Execute(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
