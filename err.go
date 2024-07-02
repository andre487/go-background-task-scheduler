package bgscheduler

import "errors"

// SchedulerError is the instance of error that identifies this package internal errors.
// SchedulerError is in the chains of all errors returned from this package
// excluding errors from Task, they returned as is.
var SchedulerError = errors.New("BG scheduler error")

// TaskErrorWrapper is the wrapper of error that can occur in a Task.
// It contains TaskName and Err.
type TaskErrorWrapper struct {
	TaskName string
	Err      error
}
