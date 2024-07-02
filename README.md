# Go background task scheduler

Package for task scheduling inside a process.
Supports interval tasks and tasks that should be executed at exact time.

Supports persistence with [bbolt](https://github.com/etcd-io/bbolt), 
intervals can be preserved between application launches.

## Installation

```shell
go get github.com/andre487/go-background-task-scheduler
```

## Documentation

[Documentation](https://pkg.go.dev/github.com/andre487/go-background-task-scheduler)
on pkg.go.dev

## Very basic example

```go
package main

import (
	"errors"
	"log"
	"time"

	bgscheduler "github.com/andre487/go-background-task-scheduler"
)

func main() {
	now := time.Now()
	after20Seconds := now.Add(20 * time.Second)

	// Scheduler config with persistence
	// Intervals will be followed after application restart
	// If DbPath is not set, Scheduler won't be persistent
	scheduler, err := bgscheduler.NewScheduler(&bgscheduler.Config{
		Logger:       log.Default(),            // Can be omitted, it's log.Default() by default
		LogLevel:     bgscheduler.LogLevelWarn, // Can be omitted, it's LogLevelWarn by default
		ScanInterval: 100 * time.Millisecond,   // Can be omitted, it's 100ms by default
		DbPath:       "/tmp/go-background-task-scheduler/examples/basic/scheduler.db",
	})
	if err != nil {
		log.Fatalf("Unable creating new scheduler: %s\n", err)
	}

	// Scheduler should be closed at the end of program execution
	defer scheduler.Close()

	// Scheduling an interval task. It will be executed every 10s
	// Its last launch time will be stored to DB,
	// and the interval will be followed after application restart
	err = scheduler.ScheduleIntervalTask("MessagePrinter", 10*time.Second, func() error {
		log.Println("Print the important message")
		return nil // No error
	})
	if err != nil {
		log.Fatalf("Unable schedule MessagePrinter: %s\n", err)
	}

	// Scheduling an exact time task
	// It will be executed every minute at Second
	// If something goes wrong, panic will be raised.
	// An error that returned from the task will be printed to log every execution
	scheduler.MustScheduleExactTimeTask(
		"MinutelyFail",
		bgscheduler.ExactLaunchTime{
			Hour:   -1,
			Minute: -1,
			Second: after20Seconds.Second(),
		},
		func() error {
			return errors.New("fiasco")
		},
	)

	// Run scheduled tasks. errChan is a nil in this case
	// errors returned by tasks will be printed to log only
	go scheduler.Run(nil)

	// Running for 5 minutes
	time.Sleep(5 * time.Minute)
}
```

## More examples

More examples can be found in [examples](examples) directory.
