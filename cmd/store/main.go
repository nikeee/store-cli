// Command store is a CLI for App Store Connect and Google Play.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/nikeee/store-cli/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Run(ctx, os.Args[1:], cli.DefaultDeps())
	stop()
	os.Exit(code)
}
