package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ubuntu/aad-auth/cmd/aad-cli/cli"
	"github.com/ubuntu/aad-auth/internal/logger"
)

func run(a *cli.App) int {
	defer installSignalHandler(a)()

	if err := a.Run(); err != nil {
		logger.Err(context.Background(), err.Error())

		if a.UsageError() {
			return 2
		}
		return 1
	}

	return 0
}

func installSignalHandler(a *cli.App) func() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			switch v, ok := <-c; v {
			case syscall.SIGINT, syscall.SIGTERM:
				if err := a.Quit(); err != nil {
					logger.Crit(context.Background(), "failed to quit: %v", err)
				}
				return
			default:
				// channel was closed: we exited
				if !ok {
					return
				}
			}
		}
	}()

	return func() {
		signal.Stop(c)
		close(c)
		wg.Wait()
	}
}

func main() {
	// Test with dummy args from the repo root (AAD setup not necessary)
	// app := cli.New(cli.WithCacheDir("./nss/testdata/users_in_db"), cli.WithConfigFile("./conf/aad.conf.template"),
	// 	cli.WithRootUID(1000), cli.WithRootGID(1000), cli.WithShadowGID(1000))

	// The real deal
	app := cli.New()

	os.Exit(run(app))
}
