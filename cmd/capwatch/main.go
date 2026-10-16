package main

import (
	"context"
	"errors"
	"os"
	"syscall"

	"charm.land/fang/v2"

	"github.com/mariyzx/capwatch/internal/cli"
)

func main() {
	err := fang.Execute(context.Background(), cli.NewCmdRoot(), fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM))

	switch {
	case err == nil:
	case errors.Is(err, context.Canceled):
		os.Exit(130)
	default:
		os.Exit(1)
	}
}
