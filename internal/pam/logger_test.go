package pam_test

import (
	"fmt"
	"testing"

	pamCom "github.com/msteinert/pam"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/pam"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		priority pam.Priority

		want string
	}{
		"new logger, debug enabled":        {priority: pam.LogDebug, want: "7: " + pam.DebugWelcome},
		"new logger, no debug, no message": {priority: pam.LogInfo, want: ""},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tx, err := pamCom.Start("", "", nil)
			require.NoError(t, err, "Setup: pam should start a transaction with no error")
			cpam := pam.Handle(tx.Handle)

			var content string
			l := pam.NewLogger(cpam, tc.priority, pam.WithPamLoggerFunc(
				func(pamh pam.Handle, priority int, format string, a ...any) {
					content += fmt.Sprintf("%d: %s", priority, fmt.Sprintf(format, a...))
				},
			))
			l.Close()

			require.Equal(t, tc.want, content, "Logged the expected content")
		})
	}
}

func TestLogging(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		logLevel  string
		withDebug bool

		want string
	}{
		"debug": {logLevel: "debug", withDebug: true, want: "7: my log message"},
		"info":  {logLevel: "info", withDebug: true, want: "6: my log message"},
		"warn":  {logLevel: "warn", withDebug: true, want: "4: my log message"},
		"err":   {logLevel: "err", withDebug: true, want: "3: my log message"},
		"crit":  {logLevel: "crit", withDebug: true, want: "2: my log message"},

		// log level
		"debug is not printed with default log level":    {logLevel: "debug", withDebug: false, want: ""},
		"info message is printed with default log level": {logLevel: "info", withDebug: false, want: "6: my log message"},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			priority := pam.LogInfo
			if tc.withDebug {
				priority = pam.LogDebug
			}

			tx, err := pamCom.Start("", "", nil)
			require.NoError(t, err, "Setup: pam should start a transaction with no error")
			cpam := pam.Handle(tx.Handle)

			var content string
			l := pam.NewLogger(cpam, priority, pam.WithPamLoggerFunc(
				func(pamh pam.Handle, priority int, format string, a ...any) {
					content += fmt.Sprintf("%d: %s", priority, fmt.Sprintf(format, a...))
				},
			))
			defer l.Close()

			switch tc.logLevel {
			case "debug":
				l.Debug("my %s message", "log")
			case "info":
				l.Info("my %s message", "log")
			case "warn":
				l.Warn("my %s message", "log")
			case "err":
				l.Err("my %s message", "log")
			case "crit":
				l.Crit("my %s message", "log")
			}

			require.Contains(t, content, tc.want, "Logged expected content")
		})
	}
}
