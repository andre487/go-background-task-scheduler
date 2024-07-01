package bgscheduler

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

const MinInterval = 250 * time.Millisecond
const DefaultScanInterval = 100 * time.Millisecond

type LogLevel uint32

// Same uint32 values that in Logrus
//
//goland:noinspection GoUnusedConst
const (
	LogLevelPanic LogLevel = iota
	LogLevelFatal
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
	LogLevelTrace
)

func (t LogLevel) String() string {
	switch t {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "TRACE"
	}
}

type Logger interface {
	Printf(format string, v ...any)
}

type Config struct {
	Logger         Logger
	LogLevel       LogLevel
	DbPath         string
	DbQueryTimeout time.Duration
	ScanInterval   time.Duration
}

type Task func() error

type Scheduler struct {
	logger       *logWrap
	db           *dbWrap
	shouldRun    bool
	running      bool
	runWg        sync.WaitGroup
	closed       bool
	scanInterval time.Duration

	existingTasks  map[string]bool
	intervalTasks  map[string]*intervalTaskData
	exactTimeTasks map[string]*exactTimeTaskData
}

func NewScheduler(conf *Config) (*Scheduler, error) {
	if conf.Logger == nil {
		conf.Logger = log.Default()
	}
	if conf.LogLevel == 0 {
		conf.LogLevel = LogLevelError
	}
	if conf.ScanInterval == 0 {
		conf.ScanInterval = DefaultScanInterval
	}

	logger := newLogWrap(conf.Logger, conf.LogLevel)

	db, err := newDbWrap(conf.DbPath, logger, conf.DbQueryTimeout)
	if err != nil {
		return nil, errors.Join(SchedulerError, err)
	}

	t := Scheduler{
		shouldRun:      true,
		scanInterval:   conf.ScanInterval,
		logger:         logger,
		db:             db,
		existingTasks:  map[string]bool{},
		intervalTasks:  map[string]*intervalTaskData{},
		exactTimeTasks: map[string]*exactTimeTaskData{},
	}

	return &t, nil
}

func MustCreateNewScheduler(conf *Config) *Scheduler {
	return must1(NewScheduler(conf))
}

func (r *Scheduler) ScheduleIntervalTask(taskName string, interval time.Duration, task Task) error {
	r.ensureNotClosed()
	if interval < MinInterval {
		interval = MinInterval
		r.logger.Warn("Interval for task %s fell back to minimum: %d", taskName, MinInterval)
	}

	if r.existingTasks[taskName] {
		return errors.Join(SchedulerError, fmt.Errorf("task %s already exists", taskName))
	}

	lastLaunchTime, err := r.getLastLaunchTime(taskName)
	if err != nil {
		return err
	}
	r.intervalTasks[taskName] = newIntervalTaskData(interval, task, lastLaunchTime)
	r.existingTasks[taskName] = true
	return nil
}

func (r *Scheduler) MustScheduleIntervalTask(taskName string, interval time.Duration, task Task) {
	must0(r.ScheduleIntervalTask(taskName, interval, task))
}

func (r *Scheduler) RemoveIntervalTask(taskName string) {
	if r.existingTasks[taskName] {
		delete(r.intervalTasks, taskName)
		delete(r.existingTasks, taskName)
	}
}

func (r *Scheduler) ScheduleExactTimeTask(taskName string, exactLaunchTime ExactLaunchTime, task Task) error {
	r.ensureNotClosed()
	if exactLaunchTime.Zero() {
		return errors.Join(SchedulerError, fmt.Errorf("zero ExactLaunchTime for task %s", taskName))
	}

	if r.existingTasks[taskName] {
		return errors.Join(SchedulerError, fmt.Errorf("task %s already exists", taskName))
	}

	lastLaunchTime, err := r.getLastLaunchTime(taskName)
	if err != nil {
		return err
	}
	r.exactTimeTasks[taskName] = newExactTimeTaskData(exactLaunchTime, task, lastLaunchTime)
	r.existingTasks[taskName] = true
	return nil
}

func (r *Scheduler) MustScheduleExactTimeTask(taskName string, exactLaunchTime ExactLaunchTime, task Task) {
	must0(r.ScheduleExactTimeTask(taskName, exactLaunchTime, task))
}

func (r *Scheduler) RemoveExactTimeTask(taskName string) {
	if r.existingTasks[taskName] {
		delete(r.exactTimeTasks, taskName)
		delete(r.existingTasks, taskName)
	}
}

func (r *Scheduler) Run(errChan chan TaskErrorWrapper) {
	if r.running {
		panic("try to Run already running Scheduler")
	}
	r.runWg.Add(1)

	r.ensureNotClosed()
	r.running = true
	r.shouldRun = true

	for r.shouldRun {
		time.Sleep(r.scanInterval)
		if !r.shouldRun {
			break
		}

		for taskName, taskData := range r.intervalTasks {
			if !r.shouldRun {
				break
			}
			r.runIntervalTask(taskName, taskData, errChan)
		}

		for taskName, taskData := range r.exactTimeTasks {
			if !r.shouldRun {
				break
			}
			r.runExactTimeTask(taskName, taskData, errChan)
		}
	}

	if errChan != nil {
		close(errChan)
	}

	r.runWg.Done()
	r.running = false
}

func (r *Scheduler) Running() bool {
	return r.running
}

func (r *Scheduler) Stop() {
	r.shouldRun = false
}

func (r *Scheduler) Wait() {
	r.runWg.Wait()
}

func (r *Scheduler) Close() {
	r.Stop()

	if r.closed {
		r.Wait()
		return
	}
	r.closed = true

	r.Wait()
	if r.db.Persistent() {
		r.db.Close()
	}
}

func (r *Scheduler) getLastLaunchTime(taskName string) (*time.Time, error) {
	lastLaunchTime := &zeroTime
	if r.db.Persistent() {
		var err error
		lastLaunchTime, err = r.db.GetLastLaunch(taskName)
		if err != nil {
			return nil, errors.Join(SchedulerError, err)
		}
	}
	return lastLaunchTime, nil
}

func (r *Scheduler) ensureNotClosed() {
	if r.closed {
		panic("try to use closed Scheduler")
	}
}

func (r *Scheduler) runIntervalTask(taskName string, taskData *intervalTaskData, errChan chan TaskErrorWrapper) {
	if time.Now().Sub(*taskData.LastLaunchTime) < taskData.Interval {
		return
	}

	r.logger.Info("Executing task %s", taskName)
	if err := taskData.Task(); err != nil {
		r.logger.Error("Error when executing task %s: %s", taskName, err)
		if errChan != nil {
			errChan <- TaskErrorWrapper{TaskName: taskName, Err: err}
		}
	}

	now := time.Now()
	taskData.LastLaunchTime = &now
	if r.db.Persistent() {
		if err := r.db.SetLastLaunch(taskName, now); err != nil {
			r.logger.Error("Error when storing last launch time for task %s: %s", taskName, err)
			if errChan != nil {
				errChan <- TaskErrorWrapper{
					TaskName: taskName,
					Err:      errors.Join(SchedulerError, err),
				}
			}
		}
	}
	r.logger.Debug("Success for task %s", taskName)
}

func (r *Scheduler) runExactTimeTask(taskName string, taskData *exactTimeTaskData, errChan chan TaskErrorWrapper) {
	if taskData.UpdateTimeToLaunch() > 0 {
		return
	}

	r.logger.Info("Executing task %s", taskName)
	if err := taskData.Task(); err != nil {
		r.logger.Error("Error when executing task %s: %s", taskName, err)
		if errChan != nil {
			errChan <- TaskErrorWrapper{TaskName: taskName, Err: err}
		}
	}

	taskData.ResetAfterLaunch()
	if r.db.Persistent() {
		if err := r.db.SetLastLaunch(taskName, time.Now()); err != nil {
			r.logger.Error("Error when storing last launch time for task %s: %s", taskName, err)
			if errChan != nil {
				errChan <- TaskErrorWrapper{
					TaskName: taskName,
					Err:      errors.Join(SchedulerError, err),
				}
			}
		}
	}
	r.logger.Debug("Success for task %s", taskName)
}
