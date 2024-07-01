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
	run          bool
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
		run:            true,
		scanInterval:   conf.ScanInterval,
		logger:         logger,
		db:             db,
		existingTasks:  map[string]bool{},
		intervalTasks:  map[string]*intervalTaskData{},
		exactTimeTasks: map[string]*exactTimeTaskData{},
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

	lastLaunchTime, err := r.getLastLaunchTime(taskName)
	if err != nil {
		return err
	}
	r.intervalTasks[taskName] = newIntervalTaskData(interval, task, lastLaunchTime)
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

func (r *Scheduler) ScheduleWithExactTime(taskName string, exactLaunchTime ExactLaunchTime, task Task) error {
	log.Fatalln("WIP")

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

func (r *Scheduler) MustScheduleWithExactTime(taskName string, exactLaunchTime ExactLaunchTime, task Task) {
	must0(r.ScheduleWithExactTime(taskName, exactLaunchTime, task))
}

func (r *Scheduler) RemoveTaskWithExactTime(taskName string) {
	if r.existingTasks[taskName] {
		delete(r.exactTimeTasks, taskName)
		delete(r.existingTasks, taskName)
	}
}

func (r *Scheduler) Run(errChan chan TaskErrorWrapper) {
	r.ensureNotClosed()

	wg := sync.WaitGroup{}
	wg.Add(2)
	r.run = true

	go func() {
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
		wg.Done()
	}()

	go func() {
		for r.run {
			time.Sleep(r.scanInterval)
			if !r.run {
				break
			}
			for taskName, taskData := range r.exactTimeTasks {
				if !r.run {
					break
				}
				r.runTaskWithExactTime(taskName, taskData, errChan)
			}
		}
		wg.Done()
	}()

	wg.Wait()
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

func (r *Scheduler) runTaskWithExactTime(taskName string, taskData *exactTimeTaskData, errChan chan TaskErrorWrapper) {
	if time.Now().Sub(*taskData.LastLaunchTime) < 0 {
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
