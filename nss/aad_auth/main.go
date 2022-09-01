package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
)

func main() {
	flag.Usage = aadAuthUsage
	flag.Parse()

	switch flag.Arg(0) {
	case "getent":
		ctx := nss.CtxWithSyslogLogger(context.Background())
		defer logger.CloseLoggerFromContext(ctx)

		db, key := flag.Arg(1), flag.Arg(2)

		if !dbIsSupported(db) {
			log.Fatalf("Request db %s is not supported", db)
		}

		out := Getent(ctx, db, key)
		fmt.Print(out)
	case "":
		flag.Usage()
		os.Exit(1)
	default:
		log.Fatalf("Invalid argument %s", flag.Arg(0))
	}
}

func dbIsSupported(db string) bool {
	supportedDbs := []string{"group", "passwd", "shadow"}
	for _, d := range supportedDbs {
		if d == db {
			return true
		}
	}
	return false
}

func aadAuthUsage() {
	fmt.Fprintln(os.Stderr, `
This executable should not be used directly, but should you wish too:

Usage: aad_auth getenv {dbName} {key}
		
dbName: Name of the database to be queried. Supported values: passwd, group, shadow
key (optional): name or uid/gid of the entry to be queried for.
  Supported keys for:
    - passwd: name, uid
    - group: name, gid
    - shadow: name`)
}
