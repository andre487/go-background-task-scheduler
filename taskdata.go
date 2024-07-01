package bgscheduler

import (
	"time"
)

type ExactLaunchTime struct {
	Hour   int
	Minute int
	Second int
}

func (r *ExactLaunchTime) Equals(other ExactLaunchTime) bool {
	return other.Hour == r.Hour && other.Minute == r.Minute && other.Second == r.Second
}

func (r *ExactLaunchTime) Zero() bool {
	return r.Hour == -1 && r.Minute == -1 && r.Second == -1
}

type intervalTaskData struct {
	Task           Task
	Interval       time.Duration
	LastLaunchTime *time.Time
}

func newIntervalTaskData(interval time.Duration, task Task, lastLaunchTime *time.Time) *intervalTaskData {
	return &intervalTaskData{
		Interval:       interval,
		Task:           task,
		LastLaunchTime: lastLaunchTime,
	}
}

type exactTimeInterval time.Duration

const (
	exactTimeDay    = exactTimeInterval(24 * time.Hour)
	exactTimeHour   = exactTimeInterval(time.Hour)
	exactTimeMinute = exactTimeInterval(time.Minute)
)

type exactTimeTaskData struct {
	ExactLaunchTime
	Task           Task
	LastLaunchTime *time.Time
	TimeToLaunch   time.Duration

	intervalType exactTimeInterval
	lastScanTime time.Time
}

func newExactTimeTaskData(exactLaunchTime ExactLaunchTime, task Task, lastLaunchTime *time.Time) *exactTimeTaskData {
	r := &exactTimeTaskData{
		ExactLaunchTime: exactLaunchTime,
		Task:            task,
		LastLaunchTime:  lastLaunchTime,
		lastScanTime:    time.Now(),
	}
	r.ResetTimeToLaunch()
	return r
}

func (r *exactTimeTaskData) ResetAfterLaunch() (timeToLaunch time.Duration) {
	n := time.Now()
	r.LastLaunchTime = &n
	return r.ResetTimeToLaunch()
}

func (r *exactTimeTaskData) ResetTimeToLaunch() (timeToLaunch time.Duration) {
	if r.ExactLaunchTime.Zero() {
		panic("unexpected zero ExactLaunchTime")
	}

	now := time.Now()

	hour := now.Hour()
	minute := now.Minute()
	second := now.Second()

	if r.Hour >= 0 {
		r.intervalType = exactTimeDay
	} else if r.Minute >= 0 {
		r.intervalType = exactTimeHour
	} else if r.Second >= 0 {
		r.intervalType = exactTimeMinute
	}
	interval := time.Duration(r.intervalType)

	switch r.intervalType {
	case exactTimeMinute:
		second = r.Second
		break
	case exactTimeHour:
		if r.Second >= 0 {
			second = r.Second
		}
		minute = r.Minute
	default:
		if r.Second >= 0 {
			second = r.Second
		}
		if r.Minute >= 0 {
			minute = r.Minute
		}
		hour = r.Hour
	}
	nextTimeToLaunch := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, time.Local)

	// Default: Call will be in the future or now
	nextTimeDelta := nextTimeToLaunch.Sub(now)
	// Call should be in the past
	if nextTimeDelta < 0 {
		// Default: Actually was
		nextTimeDelta += interval - nextTimeToLaunch.Sub(*r.LastLaunchTime)
		// Actually wasn't
		if r.LastLaunchTime.Year() <= zeroYear {
			nextTimeDelta = 0
		}
	}

	r.TimeToLaunch = nextTimeDelta
	return r.TimeToLaunch
}

func (r *exactTimeTaskData) UpdateTimeToLaunch() (timeToLaunch time.Duration) {
	now := time.Now()
	r.TimeToLaunch -= now.Sub(r.lastScanTime)
	r.lastScanTime = now
	return r.TimeToLaunch
}
