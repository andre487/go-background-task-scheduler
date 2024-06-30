package bgscheduler

import (
	"errors"
	"testing"
	"time"
)

func TestScheduler_MustScheduleWithInterval_Persistent_NoErrChan(t *testing.T) {
	dbPath := createDbPath(t)

	scheduler := MustNewScheduler(&Config{DbPath: dbPath, Logger: &logStub{}})
	defer scheduler.Close()

	var successExecTime time.Time
	var errExecTime time.Time

	scheduler.MustScheduleWithInterval("SuccessfulTask", 250*time.Millisecond, func() error {
		successExecTime = time.Now()
		return nil
	})

	scheduler.MustScheduleWithInterval("ErrTask", 250*time.Millisecond, func() error {
		errExecTime = time.Now()
		return errors.New("test error")
	})

	go scheduler.Run(nil)
	time.Sleep(300 * time.Millisecond)
	scheduler.Stop()

	now := time.Now()
	if now.Sub(successExecTime) > 250*time.Millisecond {
		t.Errorf("unexpected successExecTime diff: %s", now.Sub(successExecTime))
	}
	if now.Sub(errExecTime) > 250*time.Millisecond {
		t.Errorf("unexpected errExecTime diff: %s", now.Sub(successExecTime))
	}

	scheduler.Close()

	db := must1(newDbWrap(dbPath, createLogger(), 10*time.Second))
	successDbLastTime := must1(db.GetLastLaunch("SuccessfulTask"))
	errLastTime := must1(db.GetLastLaunch("ErrTask"))

	if now.Sub(*successDbLastTime) > 5*time.Second {
		t.Errorf("unexpected successDbLastTime: %s", successExecTime)
	}
	if now.Sub(*errLastTime) > 5*time.Second {
		t.Errorf("unexpected errLastTime: %s", errLastTime)
	}
}

func TestScheduler_MustScheduleWithInterval_NonPersistent_ErrChan(t *testing.T) {
	scheduler := MustNewScheduler(&Config{DbPath: "", Logger: &logStub{}})
	defer scheduler.Close()

	var successRunCount int
	var errRunCount int

	scheduler.MustScheduleWithInterval("SuccessfulTask", 250*time.Millisecond, func() error {
		successRunCount += 1
		return nil
	})

	scheduler.MustScheduleWithInterval("ErrTask", 430*time.Millisecond, func() error {
		errRunCount += 1
		return errors.New("test error")
	})

	errChan := make(chan TaskErrorWrapper, 5)
	go scheduler.Run(errChan)
	time.Sleep(time.Second)
	scheduler.Stop()

	if successRunCount != 3 {
		t.Errorf("unexpected successRunCount: %d", successRunCount)
	}
	if errRunCount != 2 {
		t.Errorf("unexpected errRunCount: %d", errRunCount)
	}

	chanErrCount := len(errChan)
	if chanErrCount != errRunCount {
		t.Errorf("unexpected chanErrCount: %d", chanErrCount)
	}

	for errWrapper := range errChan {
		if errWrapper.TaskName != "ErrTask" {
			t.Errorf("unexpected errWrapper.TaskName: %s", errWrapper.TaskName)
		}
		if errWrapper.Err.Error() != "test error" {
			t.Errorf("unexpected errWrapper.Err: %s", errWrapper.Err)
		}
	}
}
