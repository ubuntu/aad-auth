package nss_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/logger"
	"github.com/ubuntu/aad-auth/internal/nss"
)

func TestCtxWithSyslogLogger(t *testing.T) {
	t.Parallel()
	l := &dummyLogger{}
	ctx := nss.CtxWithSyslogLogger(context.Background(), nss.WithLogWriter(l))
	err := logger.CloseLoggerFromContext(ctx)
	require.NoError(t, err, "CloseLoggerFromContext should not error as attached to context and closing logger works")
}

func TestCtxWithSyslogLoggerDebugWithEnVariable(t *testing.T) {
	tests := map[string]struct {
		nssLogEnv string

		want string
	}{
		"log debug message when in debug mode": {nssLogEnv: "1", want: "DEBUG: nss_aad: NSS AAD DEBUG enabled\n"},
		"log to stderr when set to":            {nssLogEnv: "stderr", want: ""},
		"don't log anything when not in debug": {want: ""},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.nssLogEnv != "" {
				err := os.Setenv(nss.NssLogEnv, tc.nssLogEnv)
				require.NoError(t, err, "Setup: can’t set environment variable for debug log")
				defer func() {
					err := os.Unsetenv(nss.NssLogEnv)
					require.NoError(t, err, "Teardown: can’t restore by unsetting environment variable for debug log")
				}()
			}

			l := &dummyLogger{}
			ctx := nss.CtxWithSyslogLogger(context.Background(), nss.WithLogWriter(l))
			defer logger.CloseLoggerFromContext(ctx)

			if tc.nssLogEnv == "stderr" {
				v := ctx.Value("loggerCtxKey")
				require.Empty(t, v, "Context should not have a logger attached")
			}

			require.Equal(t, tc.want, l.content, "Should log expected debug message or nothing if nssLogEnv is not set")
		})
	}
}

func TestLogging(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		logFn      func(ctx context.Context, format string, a ...any)
		forceDebug bool

		wantLoggerPrint string
	}{
		"debug": {logFn: logger.Debug, forceDebug: true, wantLoggerPrint: "DEBUG: nss_aad: my log message\n"},
		"info":  {logFn: logger.Info, forceDebug: true, wantLoggerPrint: "INFO: nss_aad: my log message\n"},
		"warn":  {logFn: logger.Warn, forceDebug: true, wantLoggerPrint: "WARNING: nss_aad: my log message\n"},
		"err":   {logFn: logger.Err, forceDebug: true, wantLoggerPrint: "ERROR: nss_aad: my log message\n"},
		"crit":  {logFn: logger.Crit, forceDebug: true, wantLoggerPrint: "CRITICAL: nss_aad: my log message\n"},

		// log level
		"debug is not printed with default log level":    {logFn: logger.Debug, forceDebug: false, wantLoggerPrint: ""},
		"info message is printed with default log level": {logFn: logger.Info, forceDebug: false, wantLoggerPrint: "INFO: nss_aad: my log message\n"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			l := &dummyLogger{}
			opts := []nss.Option{nss.WithLogWriter(l)}
			if tc.forceDebug {
				opts = append(opts, nss.WithDebug())
			}

			ctx := nss.CtxWithSyslogLogger(context.Background(), opts...)
			defer func() { logger.CloseLoggerFromContext(ctx) }()

			tc.logFn(ctx, "my %s message", "log")

			content := l.content
			if tc.wantLoggerPrint == "" {
				require.Empty(t, content, "Should have not logged anything")
				return
			}
			require.Contains(t, content, tc.wantLoggerPrint, "Logged expected content")
		})
	}
}

type dummyLogger struct {
	content string
}

func (d *dummyLogger) Debug(msg string) error {
	d.content = fmt.Sprintf("DEBUG: %s", msg)
	return nil
}
func (d *dummyLogger) Info(msg string) error {
	d.content = fmt.Sprintf("INFO: %s", msg)
	return nil
}
func (d *dummyLogger) Warning(msg string) error {
	d.content = fmt.Sprintf("WARNING: %s", msg)
	return nil
}
func (d *dummyLogger) Err(msg string) error {
	d.content = fmt.Sprintf("ERROR: %s", msg)
	return nil
}
func (d *dummyLogger) Crit(msg string) error {
	d.content = fmt.Sprintf("CRITICAL: %s", msg)
	return nil
}
func (d dummyLogger) Close() error {
	return nil
}
