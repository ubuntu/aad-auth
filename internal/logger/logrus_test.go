package logger_test

import (
	"context"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/logger"
)

func TestCtxWithLogrusLogger(t *testing.T) {
	ctx := logger.CtxWithLogger(context.Background(), &logger.LogrusLogger{})
	err := logger.CloseLoggerFromContext(ctx)
	require.NoError(t, err, "CloseLoggerFromContext should not error as attached to context and closing logger works")
}

func TestLogrusLogging(t *testing.T) {
	tests := map[string]struct {
		logFn    func(ctx context.Context, format string, a ...any)
		loglevel int

		wantLoggerPrint string
	}{
		"debug with default verbosity": {logFn: logger.Debug},
		"debug with verbosity":         {logFn: logger.Debug, wantLoggerPrint: "DEBUG: my log message", loglevel: 2},
		"debug with caller":            {logFn: logger.Debug, wantLoggerPrint: "DEBUG:github.com/ubuntu/aad-auth/internal/logger_test.TestLogrusLogging.func1:63: my log message", loglevel: 3},
		"info with default verbosity":  {logFn: logger.Info},
		"info with verbosity":          {logFn: logger.Info, wantLoggerPrint: "INFO: my log message", loglevel: 1},
		"warning":                      {logFn: logger.Warn, wantLoggerPrint: "WARNING: my log message"},
		"error":                        {logFn: logger.Err, wantLoggerPrint: "ERROR: my log message"},

		// special cases
		"message already have an EOL": {logFn: logger.Warn, wantLoggerPrint: "WARNING: my log message\n"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			done := make(chan struct{})
			l := &logger.LogrusLogger{FieldLogger: logrus.StandardLogger()}
			logrus.SetFormatter(&logger.LogrusFormatter{})
			logger.SetVerboseMode(tc.loglevel)
			defer logrus.SetReportCaller(false)

			var contentLog []byte

			r, w := io.Pipe()
			ctx = logger.CtxWithLogger(ctx, l)
			defer logger.CloseLoggerFromContext(ctx)
			origOut := logrus.StandardLogger().Out
			logrus.StandardLogger().SetOutput(w)
			defer logrus.StandardLogger().SetOutput(origOut)
			go func() {
				defer close(done)
				var err error
				contentLog, err = io.ReadAll(r)
				require.NoError(t, err, "read from redirected output should not fail")
			}()

			tc.logFn(ctx, "my %s message", "log")

			w.Close()
			<-done

			content := string(contentLog)
			require.Contains(t, content, tc.wantLoggerPrint, "Logged expected content")
		})
	}
}
