package bgscheduler

import (
	"fmt"
	"time"
)

const zeroYear = 1970

var zeroTime = time.Date(zeroYear, 1, 1, 0, 0, 0, 0, time.UTC)

func must0(err error) {
	if err != nil {
		panic(fmt.Sprintf("unexpected error when calling required method: %s", err))
	}
}

func must1[T interface{}](val T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("unexpected error when calling required method: %s", err))
	}
	return val
}
