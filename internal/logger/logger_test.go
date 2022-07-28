package logger_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/logger"
)

func TestCtxWithLogger(t *testing.T) {
	ctx := logger.CtxWithLogger(context.Background(), &dummyLogger{})
	err := logger.CloseLoggerFromContext(ctx)
	require.NoError(t, err, "CloseLoggerFromContext should not error as attached to context and closing logger works")
}

func TestCloseLoggerFromContextNoLogger(t *testing.T) {
	err := logger.CloseLoggerFromContext(context.Background())
	require.Error(t, err, "CloseLoggerFromContext should error as context has no logger attached")
}

func TestLogging(t *testing.T) {
	tests := map[string]struct {
		logFn              func(ctx context.Context, format string, a ...any)
		hasLoggerInContext bool

		wantLoggerPrint string
	}{
		"debug, with logger": {logFn: logger.Debug, hasLoggerInContext: true, wantLoggerPrint: "DEBUG: my log message"},
		"debug, on stderr":   {logFn: logger.Debug, hasLoggerInContext: false, wantLoggerPrint: "DEBUG: my log message"},

		"info, with logger": {logFn: logger.Info, hasLoggerInContext: true, wantLoggerPrint: "INFO: my log message"},
		"info, on stderr":   {logFn: logger.Info, hasLoggerInContext: false, wantLoggerPrint: "INFO: my log message"},

		"warn, with logger": {logFn: logger.Warn, hasLoggerInContext: true, wantLoggerPrint: "WARNING: my log message"},
		"warn, on stderr":   {logFn: logger.Warn, hasLoggerInContext: false, wantLoggerPrint: "WARNING: my log message"},

		"err, with logger": {logFn: logger.Err, hasLoggerInContext: true, wantLoggerPrint: "ERROR: my log message"},
		"err, on stderr":   {logFn: logger.Err, hasLoggerInContext: false, wantLoggerPrint: "ERROR: my log message"},

		"crit, with logger": {logFn: logger.Crit, hasLoggerInContext: true, wantLoggerPrint: "CRITICAL: my log message"},
		"crit, on stderr":   {logFn: logger.Crit, hasLoggerInContext: false, wantLoggerPrint: "CRITICAL: my log message"},

		// special cases
		"message already have an EOL": {logFn: logger.Debug, hasLoggerInContext: true, wantLoggerPrint: "DEBUG: my log message\n"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			done := make(chan struct{})
			l := &dummyLogger{}
			var contentLog []byte

			r, w := io.Pipe()
			if tc.hasLoggerInContext {
				ctx = logger.CtxWithLogger(ctx, l)
				defer logger.CloseLoggerFromContext(ctx)
				close(done)
			} else {
				origOut := log.Writer()
				log.SetOutput(w)
				defer log.SetOutput(origOut)
				go func() {
					defer close(done)
					var err error
					contentLog, err = io.ReadAll(r)
					require.NoError(t, err, "read from redirected output should not fail")
				}()
			}

			tc.logFn(ctx, "my %s message", "log")

			w.Close()
			<-done

			content := l.content
			if !tc.hasLoggerInContext {
				content = string(contentLog)
			}
			require.Contains(t, content, tc.wantLoggerPrint, "Logged expected content")
			require.True(t, strings.HasSuffix(content, "\n"), "Logged message always ends with EOL")
		})
	}
}

type dummyLogger struct {
	content string
}

func (d *dummyLogger) Debug(format string, a ...any) {
	d.content = fmt.Sprintf("DEBUG: "+format, a...)
}
func (d *dummyLogger) Info(format string, a ...any) {
	d.content = fmt.Sprintf("INFO: "+format, a...)
}
func (d *dummyLogger) Warn(format string, a ...any) {
	d.content = fmt.Sprintf("WARNING: "+format, a...)
}
func (d *dummyLogger) Err(format string, a ...any) {
	d.content = fmt.Sprintf("ERROR: "+format, a...)
}
func (d *dummyLogger) Crit(format string, a ...any) {
	d.content = fmt.Sprintf("CRITICAL: "+format, a...)
}
func (d dummyLogger) Close() error {
	return nil
}
