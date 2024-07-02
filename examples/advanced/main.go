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

	// Scheduler config without persistence
	// Intervals won't be followed after application restart
	// DbPath is not set, so Scheduler won't be persistent
	scheduler := bgscheduler.MustCreateNewScheduler(&bgscheduler.Config{
		Logger:       log.Default(),             // Can be omitted, it's log.Default() by default
		LogLevel:     bgscheduler.LogLevelDebug, // Change default LogLevelError
		ScanInterval: 50 * time.Millisecond,     // Change default 100ms
		DbPath:       "",                        // DbPath is empty – Scheduler won't be persistent
	})

	// Scheduler should be closed at the end of program execution
	defer scheduler.Close()

	// Scheduling an interval task. It will be executed every 20s.
	// If something goes wrong, panic will be raised
	scheduler.MustScheduleIntervalTask("MessagePrinter", 20*time.Second, func() error {
		log.Println("Print the important message")
		return nil // No error
	})

	// Scheduling an exact time task
	// It will be executed every minute at Second
	// If something goes wrong, panic will be raised.
	// An error that returned from the task will be passed to errChan
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

	go func() {
		// Running for 5 minutes
		<-time.After(5 * time.Minute)
		// Close can be called several times
		scheduler.Close()
	}()

	// Channel for errors retrieving
	errChan := make(chan bgscheduler.TaskErrorWrapper, 128)

	// Run scheduled tasks with errChan
	go scheduler.Run(errChan)

	// Handling errors
	for errWrap := range errChan {
		log.Printf("An error was occurred in %s\n", errWrap.TaskName)
		if errors.Is(errWrap.Err, bgscheduler.SchedulerError) {
			log.Println("It's an internal Scheduler error")
		} else {
			log.Println("It's a task error")
		}
		log.Printf("The received error: %s\n", errWrap.Err)
	}
}
