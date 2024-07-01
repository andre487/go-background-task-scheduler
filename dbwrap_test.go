package bgscheduler

import (
	"testing"
	"time"
)

func TestDbWrap_LastLaunch(t *testing.T) {
	dbPath := createDbPath(t)

	db, err := newDbWrap(dbPath, createLogger(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	tm, err := db.GetLastLaunch("SomeTask")
	if err != nil {
		t.Fatal(err)
	}
	if tm.Year() != 1970 {
		t.Errorf("unexpected default time: %s", tm)
	}

	now := time.Now()
	err = db.SetLastLaunch("SomeTask", now)
	if err != nil {
		t.Fatal(err)
	}

	tm, err = db.GetLastLaunch("SomeTask")
	if err != nil {
		t.Fatal(err)
	}

	if now.Sub(*tm) > time.Second {
		t.Errorf("times doesn't match: now=%s, res=%s", now, tm)
	}
}
