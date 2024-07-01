package bgscheduler

import (
	"testing"
	"time"
)

func TestExactTimeTaskData_Init_Daily(t *testing.T) {
	now := time.Now()
	timeHoursAfter := now.Add(time.Hour)
	timeSecondBefore := now.Add(-time.Second)
	timeYesterday := now.Add(-24*time.Hour + 2*time.Second)

	var td *exactTimeTaskData

	// Launch in the future
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   timeHoursAfter.Hour(),
		Minute: timeHoursAfter.Minute(),
		Second: timeHoursAfter.Second(),
	}, task, &timeYesterday)

	if (td.TimeToLaunch - time.Hour).Abs() > time.Minute {
		t.Errorf("unexpected time for launch in the future: %s", td.TimeToLaunch)
	}

	// The Last launch time was in ancient times
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   timeSecondBefore.Hour(),
		Minute: timeSecondBefore.Minute(),
		Second: timeSecondBefore.Second(),
	}, task, &zeroTime)

	if td.TimeToLaunch != 0 {
		t.Errorf("unexpected time for launch after ancient times: %s", td.TimeToLaunch)
	}

	// The Last launch time was today
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   timeSecondBefore.Hour(),
		Minute: -1, // Will be ignored and fallen back to 0
		Second: -1, // Will be ignored and fallen back to 0
	}, task, &timeSecondBefore)

	if (td.TimeToLaunch - 24*time.Hour).Abs() > time.Minute {
		t.Errorf("unexpected time for launch after today launch: %s", td.TimeToLaunch)
	}

	// The Last launch time was yesterday but inaccurate
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   timeSecondBefore.Hour(),
		Minute: timeSecondBefore.Minute(),
		Second: timeSecondBefore.Second(),
	}, task, &timeYesterday)

	if td.TimeToLaunch < time.Second || td.TimeToLaunch >= 4*time.Second {
		t.Errorf("unexpected time for launch after yesterday launch: %s", td.TimeToLaunch)
	}
}

func TestExactTimeTaskData_Init_Hourly(t *testing.T) {
	now := time.Now()
	timeMinuteAfter := now.Add(time.Minute)
	timeSecondBefore := now.Add(-time.Second)
	timeHourBefore := now.Add(-time.Hour + 2*time.Second)
	timeYesterday := now.Add(-24*time.Hour + 2*time.Second)

	var td *exactTimeTaskData

	// Launch in the future
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   -1,
		Minute: timeMinuteAfter.Minute(),
		Second: timeMinuteAfter.Second(),
	}, task, &timeYesterday)

	if (td.TimeToLaunch - time.Minute).Abs() > time.Second {
		t.Errorf("unexpected time for launch in the future: %s", td.TimeToLaunch)
	}

	// The Last launch time was in ancient times
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   -1,
		Minute: timeSecondBefore.Minute(),
		Second: timeSecondBefore.Second(),
	}, task, &zeroTime)

	if td.TimeToLaunch != 0 {
		t.Errorf("unexpected time for launch after ancient times: %s", td.TimeToLaunch)
	}

	// The Last launch time in the interval
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   -1,
		Minute: timeSecondBefore.Minute(),
		Second: -1, // Will be ignored and fallen back to 0
	}, task, &timeSecondBefore)

	if (td.TimeToLaunch - time.Hour).Abs() > 30*time.Second {
		t.Errorf("unexpected time for launch after today launch: %s", td.TimeToLaunch)
	}

	// The Last launch time was the hour before
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   -1,
		Minute: timeSecondBefore.Minute(),
		Second: timeSecondBefore.Second(),
	}, task, &timeHourBefore)

	if td.TimeToLaunch < time.Second || td.TimeToLaunch >= 4*time.Second {
		t.Errorf("unexpected time for launch after yesterday launch: %s", td.TimeToLaunch)
	}
}

func TestExactTimeTaskData_Init_Minutely(t *testing.T) {
	now := time.Now()
	timeSecondAfter := now.Add(time.Second)
	timeSecondBefore := now.Add(-time.Second)
	timeYesterday := now.Add(-24*time.Hour + 2*time.Second)

	var td *exactTimeTaskData

	// Launch in the future
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   -1,
		Minute: -1,
		Second: timeSecondAfter.Second(),
	}, task, &timeYesterday)

	if (td.TimeToLaunch - time.Second).Abs() > time.Minute {
		t.Errorf("unexpected time for launch in the future: %s", td.TimeToLaunch)
	}

	// The Last launch time was in ancient times
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   -1,
		Minute: -1,
		Second: timeSecondBefore.Second(),
	}, task, &zeroTime)

	if td.TimeToLaunch != 0 {
		t.Errorf("unexpected time for launch after ancient times: %s", td.TimeToLaunch)
	}

	// The Last launch time was the second before
	td = newExactTimeTaskData(ExactLaunchTime{
		Hour:   -1,
		Minute: -1,
		Second: 0,
	}, task, &timeSecondBefore)

	if (td.TimeToLaunch - time.Minute).Abs() > time.Minute {
		t.Errorf("unexpected time for launch after today launch: %s", td.TimeToLaunch)
	}
}

func task() error {
	return nil
}
