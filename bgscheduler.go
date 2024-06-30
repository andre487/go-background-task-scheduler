package bgscheduler

import (
	"errors"
	"fmt"
	"log"
	"time"
)

const MinInterval = 250 * time.Millisecond
const DefaultScanInterval = 100 * time.Millisecond

type ExactLaunchTime struct {
	Hour   int
	Minute int
	Second int
}

func (r *ExactLaunchTime) Equals(other ExactLaunchTime) bool {
	return other.Hour == r.Hour && other.Minute == r.Minute && other.Second == r.Second
}

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

type intervalTaskData struct {
	Task           Task
	Interval       time.Duration
	LastLaunchTime *time.Time
}

type Scheduler struct {
	logger        *logWrap
	db            *dbWrap
	run           bool
	closed        bool
	scanInterval  time.Duration
	existingTasks map[string]bool
	intervalTasks map[string]*intervalTaskData
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
		run:           true,
		scanInterval:  conf.ScanInterval,
		logger:        logger,
		db:            db,
		existingTasks: map[string]bool{},
		intervalTasks: map[string]*intervalTaskData{},
	}

	return &t, nil
}

func MustNewScheduler(conf *Config) *Scheduler {
	return must1(NewScheduler(conf))
}

func (r *Scheduler) ScheduleWithInterval(taskName string, interval time.Duration, task Task) error {
	r.ensureNotClosed()
	if interval < MinInterval {
		interval = MinInterval
		r.logger.Warn("Interval for task %s fell back to minimum: %d", taskName, MinInterval)
	}

	if r.existingTasks[taskName] {
		return errors.Join(SchedulerError, fmt.Errorf("task %s already exists", taskName))
	}

	taskData := &intervalTaskData{
		Task:           task,
		Interval:       interval,
		LastLaunchTime: &zeroTime,
	}

	if r.db.Persistent() {
		var err error
		taskData.LastLaunchTime, err = r.db.GetLastLaunch(taskName)
		if err != nil {
			return errors.Join(SchedulerError, err)
		}
	}

	r.intervalTasks[taskName] = taskData
	r.existingTasks[taskName] = true
	return nil
}

func (r *Scheduler) MustScheduleWithInterval(taskName string, interval time.Duration, task Task) {
	must0(r.ScheduleWithInterval(taskName, interval, task))
}

func (r *Scheduler) RemoveTaskWithInterval(taskName string) {
	if r.existingTasks[taskName] {
		delete(r.intervalTasks, taskName)
		delete(r.existingTasks, taskName)
	}
}

func (r *Scheduler) Run(errChan chan TaskErrorWrapper) {
	r.ensureNotClosed()
	r.run = true
	for r.run {
		time.Sleep(r.scanInterval)
		if !r.run {
			break
		}

		for taskName, taskData := range r.intervalTasks {
			if !r.run {
				break
			}
			r.runTaskWithInterval(taskName, taskData, errChan)
		}
	}

	if errChan != nil {
		close(errChan)
	}
}

func (r *Scheduler) Stop() {
	r.run = false
}

func (r *Scheduler) Close() {
	if r.closed {
		return
	}

	r.closed = true
	r.Stop()
	if r.db.Persistent() {
		r.db.Close()
	}
}

func (r *Scheduler) ensureNotClosed() {
	if r.closed {
		panic("try to use closed Scheduler")
	}
}

func (r *Scheduler) runTaskWithInterval(taskName string, taskData *intervalTaskData, errChan chan TaskErrorWrapper) {
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
