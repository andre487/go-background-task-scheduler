package bgscheduler

import (
	"fmt"
	"testing"
)

func TestLogWrap_Debug(t *testing.T) {
	lStub := &logStub{}
	lWrap := newLogWrap(lStub, LogLevelDebug)

	lWrap.Error("Bad thing: %s", "all have failed!")
	lWrap.Warn("Bad thing, but let it be: %s", "something has failed!")
	lWrap.Info("Something important: %s\n", "some task finished with result 0")
	lWrap.Debug("Something needed only for debug: %s", "TaskId=487")

	lCalls := lStub.Calls()
	expectedLines := []string{
		"ERROR Bad thing: all have failed!\n",
		"WARN Bad thing, but let it be: something has failed!\n",
		"INFO Something important: some task finished with result 0\n",
		"DEBUG Something needed only for debug: TaskId=487\n",
	}
	matchCollectionsLen(t, lCalls, expectedLines)
	for i := 0; i < len(expectedLines); i++ {
		call := lCalls[i]
		expectedLine := expectedLines[i]
		checkLogLine(t, call, expectedLine)
	}
}

func TestLogWrap_Info(t *testing.T) {
	lStub := &logStub{}
	lWrap := newLogWrap(lStub, LogLevelInfo)

	lWrap.Error("Bad thing: %s", "all have failed!")
	lWrap.Info("Something important: %s\n", "some task finished with result 0")
	lWrap.Debug("Something needed only for debug: %s", "TaskId=487")

	lCalls := lStub.Calls()
	expectedLines := []string{
		"ERROR Bad thing: all have failed!\n",
		"INFO Something important: some task finished with result 0\n",
	}
	matchCollectionsLen(t, lCalls, expectedLines)
	for i := 0; i < len(expectedLines); i++ {
		call := lCalls[i]
		expectedLine := expectedLines[i]
		checkLogLine(t, call, expectedLine)
	}
}

func TestLogWrap_Error(t *testing.T) {
	lStub := &logStub{}
	lWrap := newLogWrap(lStub, LogLevelError)

	lWrap.Error("Bad thing: %s", "all have failed!")
	lWrap.Info("Something important: %s\n", "some task finished with result 0")
	lWrap.Debug("Something needed only for debug: %s", "TaskId=487")

	lCalls := lStub.Calls()
	expectedLines := []string{
		"ERROR Bad thing: all have failed!\n",
	}
	matchCollectionsLen(t, lCalls, expectedLines)
	for i := 0; i < len(expectedLines); i++ {
		call := lCalls[i]
		expectedLine := expectedLines[i]
		checkLogLine(t, call, expectedLine)
	}
}

func (l *logStub) Printf(format string, v ...interface{}) {
	l.calls = append(l.calls, logStubCall{format: format, v: v})
}

func (l *logStub) Calls() []logStubCall {
	return l.calls
}

func matchCollectionsLen(t *testing.T, calls []logStubCall, expectedLines []string) {
	callCount := len(calls)
	expectedLinesCount := len(expectedLines)
	if callCount != expectedLinesCount {
		t.Fatalf(
			"call count doesn't match expected lines count: callCount=%d, expectedLinesCount=%d",
			callCount,
			expectedLinesCount,
		)
	}
}

func checkLogLine(t *testing.T, call logStubCall, expectedLine string) {
	actual := fmt.Sprintf(call.format, call.v...)
	if actual != expectedLine {
		t.Errorf("log lines don't match:\n\texpected=\"%s\"\n\tactual=\"%s\"", expectedLine, actual)
	}
}
