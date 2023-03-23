//nolint:revive // This package contains functions that are exported to C, we need the arg names.
package main

/*
#cgo LDFLAGS: -lpam -fPIC
#include <security/pam_appl.h>
#include <security/pam_ext.h>
#include <stdlib.h>
#include <string.h>

char *string_from_argv(int i, char **argv);
*/
import "C"
import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/ubuntu/aad-auth/internal/cache"
	"github.com/ubuntu/aad-auth/internal/consts"
	"github.com/ubuntu/aad-auth/internal/i18n"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/pam"
)

//go:generate sh -c "go build -ldflags='-extldflags -Wl,-soname,pam_aad.so' -buildmode=c-shared -o pam_aad.so"

var (
	opts         []pam.Option
	logsOnStderr bool
)

//export pam_sm_authenticate
func pam_sm_authenticate(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {
	// Initialize localization
	i18n.InitI18nDomain(consts.TEXTDOMAIN)

	// Attach logger and info handler.
	ctx := pam.CtxWithPamh(context.Background(), pam.Handle(pamh))
	pamLogger := pam.NewLogger(pam.Handle(pamh), pam.LogInfo)

	// Get options.
	conf := consts.DefaultConfigPath
	for _, arg := range sliceFromArgv(argc, argv) {
		opt, optarg, _ := strings.Cut(arg, "=")
		switch opt {
		case "conf":
			conf = optarg
		case "debug":
			pamLogger = pam.NewLogger(pam.Handle(pamh), pam.LogDebug)
			pamLogger.Debug("PAM AAD DEBUG enabled")
		default:
			// we have additional supported option when built for integration tests
			if supportedOption(&pamLogger, opt, optarg) {
				continue
			}
			pamLogger.Warn(i18n.G("unknown option: %s\n"), opt)
		}
	}
	if !logsOnStderr {
		ctx = logger.CtxWithLogger(ctx, pamLogger)
		defer logger.CloseLoggerFromContext(ctx)
	}

	username, err := getUser(pamh)
	if err != nil {
		pamLogger.Err(err.Error())
		return C.PAM_SYSTEM_ERR
	}
	password, err := getPassword(pamh)
	if err != nil {
		pamLogger.Err(err.Error())
		return C.PAM_SYSTEM_ERR
	}

	if err := pam.Authenticate(ctx, username, password, conf, opts...); err != nil {
		if errors.Is(err, pam.ErrPamSystem) {
			return C.PAM_SYSTEM_ERR
		}
		if errors.Is(err, pam.ErrPamAuth) {
			return C.PAM_AUTH_ERR
		}
		if errors.Is(err, pam.ErrPamIgnore) {
			return C.PAM_IGNORE
		}
	}
	return C.PAM_SUCCESS
}

//export pam_sm_setcred
func pam_sm_setcred(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {
	return C.PAM_IGNORE
}

//export pam_sm_open_session
func pam_sm_open_session(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {
	return C.PAM_SUCCESS
}

//export pam_sm_close_session
func pam_sm_close_session(pamh *C.pam_handle_t, flags, argc C.int, argv **C.char) C.int {
	return C.PAM_SUCCESS
}

func main() {
	c, err := cache.New(context.Background(), cache.WithCacheDir("../cache"), cache.WithRootUID(1000), cache.WithRootGID(1000), cache.WithShadowGID(1000))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close(context.Background())

	for u, pass := range map[string]string{
		"alice":             "alice pass",
		"bob@example.com":   "bob pass",
		"carol@example.com": "carol pass",
	} {
		if err := c.Update(context.Background(), u, pass, "", ""); err != nil {
			log.Fatal(err)
		}
	}
}
