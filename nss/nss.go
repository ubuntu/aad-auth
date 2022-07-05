package main

import (
	"context"
	"fmt"
	"log/syslog"
	"os"

	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
)

const (
	nssLogKey = "NSS_AAD_DEBUG"
)

// ctxWithSyslogLogger attach a logger to the context and set priority based on environment.
func ctxWithSyslogLogger(ctx context.Context) context.Context {
	priority := syslog.LOG_INFO
	if os.Getenv(nssLogKey) != "" {
		priority = syslog.LOG_DEBUG
	}
	nssLogger, err := nss.NewLogger(priority)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: can't find syslog to write to. Default to stderr\n")
		return ctx
	}

	return logger.CtxWithLogger(ctx, nssLogger)
}

func main() {

}
