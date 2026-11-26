package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"regclean/internal/pathcheck"
	"regclean/internal/reg"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	home, _ := os.UserHomeDir()
	code := run(ctx, os.Args[1:], deps{
		store:    reg.NewStore(),
		fs:       pathcheck.NewOSFS(),
		elevated: reg.IsElevated(),
		home:     home,
		now:      time.Now,
	}, os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
