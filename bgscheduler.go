// Package bgscheduler implements background task scheduler.
// The [NewScheduler] function creates a new scheduler with [Config] params.
// The [MustCreateNewScheduler] function does the same, but doesn't return an error,
// but panics when error occurs.
// Internal errors of package have SchedulerError error instance in the chain.
package bgscheduler

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// MinInterval is the Duration value that is minimal for interval tasks scheduling
const MinInterval = 250 * time.Millisecond

// DefaultScanInterval is the default interval for Scheduler to check the task list
const DefaultScanInterval = 100 * time.Millisecond

// LogLevel is the same uint32 values that in Logrus
type LogLevel uint32

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

// String is a function for getting string representation of log level
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

// Logger interface describes needed part of general logger that needed by Scheduler
type Logger interface {
	Printf(format string, v ...any)
}

// Config is params struct for Scheduler
type Config struct {
	Logger       Logger        // Logger should implement Logger interface: log.Default(), logrus.StandardLogger() or another
	LogLevel     LogLevel      // LogLevel defines level of logging fot the package, log.Printf will show all the messages
	DbPath       string        // DbPath is the path for the bbolt DB file. If empty string, Scheduler is not persistent
	ScanInterval time.Duration // ScanInterval defines an interval for scanning task lists
}

// Task is the interface for scheduling task
type Task func() error

// Scheduler is the crucial thing of the package. It should be created by NewScheduler or MustCreateNewScheduler
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

// NewScheduler produces Scheduler instance, returns pointer to this and an error if occurred
func NewScheduler(conf *Config) (*Scheduler, error) {
	if conf == nil {
		conf = &Config{}
	}
	if conf.Logger == nil {
		conf.Logger = log.Default()
	}
	if conf.LogLevel == 0 {
		conf.LogLevel = LogLevelWarn
	}
	if conf.ScanInterval == 0 {
		conf.ScanInterval = DefaultScanInterval
	}

	logger := newLogWrap(conf.Logger, conf.LogLevel)

	db, err := newDbWrap(conf.DbPath, logger)
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

// MustCreateNewScheduler is a version of NewScheduler that panics instead of returning errors
func MustCreateNewScheduler(conf *Config) *Scheduler {
	return must1(NewScheduler(conf))
}

// ScheduleIntervalTask function schedules a function with Task interface
// that will be called in loop with an interval.
// If an interval is less than MinInterval, it will fall back to MinInterval
// taskName should be unique for all the tasks, interval and exact time.
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

// MustScheduleIntervalTask is the version of ScheduleIntervalTask that panics instead of returning errors
func (r *Scheduler) MustScheduleIntervalTask(taskName string, interval time.Duration, task Task) {
	must0(r.ScheduleIntervalTask(taskName, interval, task))
}

// RemoveIntervalTask removes task from interval tasks queue
func (r *Scheduler) RemoveIntervalTask(taskName string) {
	if r.existingTasks[taskName] {
		delete(r.intervalTasks, taskName)
		delete(r.existingTasks, taskName)
	}
}

// ScheduleExactTimeTask function schedules a task with Task interface
// that should be executed in the exact time.
// If the previous call was skipped, the task will be executed at the nearest time.
// The Next execution will be at exact time.
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

// MustScheduleExactTimeTask is ScheduleExactTimeTask version that panics instead of returning errors
func (r *Scheduler) MustScheduleExactTimeTask(taskName string, exactLaunchTime ExactLaunchTime, task Task) {
	must0(r.ScheduleExactTimeTask(taskName, exactLaunchTime, task))
}

// RemoveExactTimeTask removes a task from exact time tasks queue
func (r *Scheduler) RemoveExactTimeTask(taskName string) {
	if r.existingTasks[taskName] {
		delete(r.exactTimeTasks, taskName)
		delete(r.existingTasks, taskName)
	}
}

// Run starts to handle tasks.
// Should be called in the goroutine.
// errChan is an optional param (can be nil) that can be used for
// retrieving errors from tasks
// Can't be called several times, on second call panic will be raised.
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

// Running tells if queues in the handling cycle
func (r *Scheduler) Running() bool {
	return r.running
}

// Stop tasks handling after Run.
// Run can be called after this method.
// It launches task handling again.
// Can be called several times.
func (r *Scheduler) Stop() {
	r.shouldRun = false
}

// Wait function locks a thread until Run finished
func (r *Scheduler) Wait() {
	r.runWg.Wait()
}

// Close stops scheduler, closes DB and waits for execution stop.
// Can be called several times.
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
	lastLaunchTime := &ZeroTime
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
