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

func TestDbWrap_ExactTimeConfig(t *testing.T) {
	dbPath := createDbPath(t)

	db, err := newDbWrap(dbPath, createLogger(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	tConf, err := db.GetExactTimeConfig("SomeTask")
	if tConf != nil {
		t.Errorf("unexpected default exec time config: %v", tConf)
	}

	conf := ExactLaunchTime{Hour: -1, Minute: 0, Second: 30}
	err = db.SetExactTimeConfig("SomeTask", conf)
	if err != nil {
		t.Fatal(err)
	}

	tConf, err = db.GetExactTimeConfig("SomeTask")
	if err != nil {
		t.Fatal(err)
	}

	if !conf.Equals(*tConf) {
		t.Errorf("configs doesn't match: original=%+v, fromQuery=%+v", conf, tConf)
	}
}
