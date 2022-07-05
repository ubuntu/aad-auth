package main

import (
	"context"
	"fmt"
	"log/syslog"
	"os"

	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
)

// ctxWithSyslogLogger attach a logger to the context and set default priority.
func ctxWithSyslogLogger(ctx context.Context) context.Context {
	nssLogger, err := nss.NewLogger(syslog.LOG_DEBUG)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: can't find syslog to write to. Default to stderr\n")
		return ctx
	}

	return logger.CtxWithLogger(ctx, nssLogger)
}

func main() {

}
