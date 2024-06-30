package bgscheduler

import (
	"log"
	"os"
	"path"
	"testing"
	"time"
)

func TestDbWrap_Basic(t *testing.T) {
	dbDir := createDbPath(t)
	defer rmDb(t, dbDir)

	db, err := newDbWrap(dbDir, createLogger(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	log.Println(db)
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
