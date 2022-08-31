package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
)

func main() {
	flag.Parse()

	if len(flag.Args()) < 2 {
		log.Fatal("Not enough arguments.")
	}

	switch flag.Arg(0) {
	case "getent":
		ctx := nss.CtxWithSyslogLogger(context.Background())
		defer logger.CloseLoggerFromContext(ctx)

		db, key := flag.Arg(1), flag.Arg(2)
		entries, err := GetEnt(ctx, db, key)

		fmt.Print(fmtOutput(ctx, entries, err))
	}
}

func fmtOutput(ctx context.Context, entries []string, err error) string {
	var out string

	status, errno := 1, 0
	if err != nil {
		status, errno = errToCStatus(ctx, err)
	}
	out = fmt.Sprintf("%d:%d\n", status, errno)

	for _, entry := range entries {
		out += (entry + "\n")
	}

	return out
}
