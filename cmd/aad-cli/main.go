// Package main implements the main entry point for the aad-cli command.
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

//go:generate go run ../generate_completion_documentation.go completion ../../generated
//go:generate go run ../generate_completion_documentation.go man ../../generated

func run(a *cli.App) int {
	defer installSignalHandler(a)()

	if err := a.Run(); err != nil {
		if a.UsageError() {
			return 2
		}
		logger.Err(context.Background(), err.Error())
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
	app := cli.New()

	os.Exit(run(app))
}
