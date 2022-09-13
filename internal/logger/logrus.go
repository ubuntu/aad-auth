package logger

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/ubuntu/aad-auth/internal/consts"
)

// LogrusLogger is a logrus-backed logger.
type LogrusLogger struct {
	logrus.FieldLogger
}

// Debug sends a debug level message to the logger.
func (l LogrusLogger) Debug(format string, a ...any) {
	l.Debugf(format, a...)
}

// Info sends an informational message to the logger.
func (l LogrusLogger) Info(format string, a ...any) {
	l.Infof(format, a...)
}

// Warn sends a warning level message to the logger.
func (l LogrusLogger) Warn(format string, a ...any) {
	l.Warningf(format, a...)
}

// Err sends an error level message to the logger.
func (l LogrusLogger) Err(format string, a ...any) {
	l.Errorf(format, a...)
}

// Crit sends a fatal message to the logger.
func (l LogrusLogger) Crit(format string, a ...any) {
	l.Fatalf(format, a...)
}

// Close is a no-op for logrus.
func (l LogrusLogger) Close() error { return nil } // no-op

// LogrusFormatter implements the logrus.Formatter interface.
type LogrusFormatter struct{}

// Format formats the log entry similar to the builtin log package.
func (f *LogrusFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer

	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	timestampFormat := "2006/01/02 15:04:05"

	b.WriteString(entry.Time.Format(timestampFormat))
	b.WriteByte(' ')
	b.WriteString(strings.ToUpper(entry.Level.String()))

	if logrus.StandardLogger().ReportCaller {
		// We have to go up the stack to get the actual function name.
		// Maybe not the best way, but this gets us out of logrus land.
		pc, _, line, ok := runtime.Caller(9)
		caller := runtime.FuncForPC(pc)

		if ok {
			b.WriteString(fmt.Sprintf(":%s:%d", caller.Name(), line))
		}
	}
	b.WriteString(": ")

	if entry.Message != "" {
		b.WriteString(entry.Message)
	}
	return b.Bytes(), nil
}

// SetVerboseMode changes the error format and logs between very, middly and non verbose.
func SetVerboseMode(level int) {
	switch level {
	case 0:
		logrus.SetLevel(consts.DefaultLogLevel)
	case 1:
		logrus.SetLevel(logrus.InfoLevel)
	case 3:
		logrus.SetReportCaller(true)
		fallthrough
	default:
		logrus.SetLevel(logrus.DebugLevel)
	}
}
