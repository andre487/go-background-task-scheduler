package bgscheduler

import "errors"

var SchedulerError = errors.New("BG scheduler error")

type TaskErrorWrapper struct {
	TaskName string
	Err      error
}
