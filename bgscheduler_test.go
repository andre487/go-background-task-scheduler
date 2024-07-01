package bgscheduler

import (
	"errors"
	"testing"
	"time"
)

func TestScheduler_MustScheduleIntervalTask_Persistent_NoErrChan(t *testing.T) {
	dbPath := createDbPath(t)

	scheduler := MustCreateNewScheduler(&Config{DbPath: dbPath, Logger: &logStub{}})
	defer scheduler.Close()

	var successExecTime time.Time
	var errExecTime time.Time

	scheduler.MustScheduleIntervalTask("SuccessfulTask", 250*time.Millisecond, func() error {
		successExecTime = time.Now()
		return nil
	})

	scheduler.MustScheduleIntervalTask("ErrTask", 250*time.Millisecond, func() error {
		errExecTime = time.Now()
		return errors.New("test error")
	})

	go scheduler.Run(nil)
	time.Sleep(300 * time.Millisecond)
	scheduler.Close()

	now := time.Now()
	if now.Sub(successExecTime) > 250*time.Millisecond {
		t.Errorf("unexpected successExecTime diff: %s", now.Sub(successExecTime))
	}
	if now.Sub(errExecTime) > 250*time.Millisecond {
		t.Errorf("unexpected errExecTime diff: %s", now.Sub(successExecTime))
	}

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

func TestScheduler_MustScheduleIntervalTask_NonPersistent_ErrChan(t *testing.T) {
	scheduler := MustCreateNewScheduler(&Config{DbPath: "", Logger: &logStub{}})
	defer scheduler.Close()

	var successRunCount int
	var errRunCount int

	scheduler.MustScheduleIntervalTask("SuccessfulTask", 250*time.Millisecond, func() error {
		successRunCount += 1
		return nil
	})

	scheduler.MustScheduleIntervalTask("ErrTask", 430*time.Millisecond, func() error {
		errRunCount += 1
		return errors.New("test error")
	})

	errChan := make(chan TaskErrorWrapper, 5)
	go scheduler.Run(errChan)
	time.Sleep(time.Second)
	scheduler.Close()

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

func TestScheduler_MustScheduleExactTimeTask_Persistent_NoErrChan(t *testing.T) {
	dbPath := createDbPath(t)

	scheduler := MustCreateNewScheduler(&Config{DbPath: dbPath, Logger: &logStub{}})
	defer scheduler.Close()

	now0 := time.Now()
	launchTime1 := now0.Add(time.Second)
	launchTime2 := now0.Add(2 * time.Second)

	var successExecTime time.Time
	var errExecTime time.Time

	scheduler.MustScheduleExactTimeTask("SuccessfulTask", ExactLaunchTime{launchTime1.Hour(), launchTime1.Minute(), launchTime1.Second()}, func() error {
		successExecTime = time.Now()
		return nil
	})

	scheduler.MustScheduleExactTimeTask("ErrTask", ExactLaunchTime{launchTime2.Hour(), launchTime2.Minute(), launchTime2.Second()}, func() error {
		errExecTime = time.Now()
		return errors.New("test error")
	})

	go scheduler.Run(nil)
	time.Sleep(3 * time.Second)
	scheduler.Close()

	now1 := time.Now()
	if now1.Sub(successExecTime) > 3*time.Second {
		t.Errorf("unexpected successExecTime diff: %s", now1.Sub(successExecTime))
	}
	if now1.Sub(errExecTime) > 3*time.Second {
		t.Errorf("unexpected errExecTime diff: %s", now1.Sub(successExecTime))
	}

	db := must1(newDbWrap(dbPath, createLogger(), 10*time.Second))
	successDbLastTime := must1(db.GetLastLaunch("SuccessfulTask"))
	errLastTime := must1(db.GetLastLaunch("ErrTask"))

	if now1.Sub(*successDbLastTime) > 5*time.Second {
		t.Errorf("unexpected successDbLastTime: %s", successExecTime)
	}
	if now1.Sub(*errLastTime) > 5*time.Second {
		t.Errorf("unexpected errLastTime: %s", errLastTime)
	}
}

func TestScheduler_MustScheduleExactTimeTask_NonPersistent_ErrChan(t *testing.T) {
	dbPath := createDbPath(t)

	scheduler := MustCreateNewScheduler(&Config{DbPath: dbPath, Logger: &logStub{}})
	defer scheduler.Close()

	now0 := time.Now()
	launchTime := now0.Add(2 * time.Second)

	scheduler.MustScheduleExactTimeTask("ErrTask", ExactLaunchTime{launchTime.Hour(), launchTime.Minute(), launchTime.Second()}, func() error {
		return errors.New("test error")
	})

	errChan := make(chan TaskErrorWrapper, 5)
	go scheduler.Run(errChan)

	time.Sleep(2 * time.Second)
	if !scheduler.Running() {
		t.Fatalf("schefuler is not running")
	}
	scheduler.Close()

	if scheduler.Running() {
		t.Fatalf("schefuler is running")
	}

	chanErrCount := len(errChan)
	if chanErrCount != 1 {
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

func TestScheduler_MixedScheduling(t *testing.T) {
	dbPath := createDbPath(t)

	scheduler := MustCreateNewScheduler(&Config{DbPath: dbPath, Logger: &logStub{}})
	defer scheduler.Close()

	now0 := time.Now()
	launchTime := now0.Add(2 * time.Second)

	scheduler.MustScheduleIntervalTask("ErrTask1", 500*time.Millisecond, func() error {
		return errors.New("test error 1")
	})

	scheduler.MustScheduleExactTimeTask("ErrTask2", ExactLaunchTime{launchTime.Hour(), launchTime.Minute(), launchTime.Second()}, func() error {
		return errors.New("test error 2")
	})

	errChan := make(chan TaskErrorWrapper, 5)
	go scheduler.Run(errChan)

	time.Sleep(2 * time.Second)
	scheduler.Close()

	chanErrCount := len(errChan)
	if chanErrCount != 5 {
		t.Errorf("unexpected chanErrCount: %d", chanErrCount)
	}

	for errWrapper := range errChan {
		if errWrapper.TaskName != "ErrTask1" && errWrapper.TaskName != "ErrTask2" {
			t.Errorf("unexpected errWrapper.TaskName: %s", errWrapper.TaskName)
		}
		if errWrapper.Err.Error() != "test error 1" && errWrapper.Err.Error() != "test error 2" {
			t.Errorf("unexpected errWrapper.Err: %s", errWrapper.Err)
		}
	}
}
