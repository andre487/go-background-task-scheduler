package bgscheduler

import (
	"os"
	"path"
	"testing"
	"time"
)

func TestDbWrap_LastLaunch(t *testing.T) {
	dbDir := createDbPath(t)
	defer rmDb(t, dbDir)

	db, err := newDbWrap(dbDir, createLogger(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	err = db.SetLastLaunch("SomeTask", now)
	if err != nil {
		t.Fatal(err)
	}

	tm, err := db.GetLastLaunch("SomeTask")
	if err != nil {
		t.Fatal(err)
	}

	if now.Sub(*tm) > time.Second {
		t.Errorf("times doesn't match: now=%s, res=%s", now, tm)
	}
}

func createDbPath(t *testing.T) string {
	dbDir, err := os.MkdirTemp(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	return path.Join(dbDir, "db.sqlite")
}

func rmDb(t *testing.T, dbPath string) {
	dbDir := path.Dir(dbPath)
	err := os.RemoveAll(dbDir)
	if err != nil {
		t.Error(err)
	}
}

func createLogger() *logWrap {
	return newLogWrap(nil, LogLevelDebug)
}
