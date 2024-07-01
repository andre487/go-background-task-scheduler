package bgscheduler

import (
	"path"
	"testing"
)

type logStubCall struct {
	format string
	v      []interface{}
}

type logStub struct {
	calls []logStubCall
}

func createLogger() *logWrap {
	return newLogWrap(nil, LogLevelDebug)
}

func createDbPath(t *testing.T) string {
	dbDir := t.TempDir()
	return path.Join(dbDir, "data.db")
}
