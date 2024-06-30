package bgscheduler

import "strings"

type logWrap struct {
	logger   Logger
	logLevel LogLevel
}

func newLogWrap(logger Logger, logLevel LogLevel) *logWrap {
	return &logWrap{logger: logger, logLevel: logLevel}
}

func (t *logWrap) Error(format string, v ...any) {
	t.log(LogLevelError, format, v...)
}

func (t *logWrap) Warn(format string, v ...any) {
	t.log(LogLevelWarn, format, v...)
}

func (t *logWrap) Info(format string, v ...any) {
	t.log(LogLevelInfo, format, v...)
}

func (t *logWrap) Debug(format string, v ...any) {
	t.log(LogLevelDebug, format, v...)
}

func (t *logWrap) log(level LogLevel, format string, v ...any) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	levelStr := level.String()
	if !strings.HasPrefix(format, levelStr) {
		format = levelStr + " " + format
	}

	if level <= t.logLevel {
		t.logger.Printf(format, v...)
	}
}
