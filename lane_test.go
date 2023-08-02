package lane

import (
	"bytes"
	"context"
	"log"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

type testKeyType string

const test_str testKeyType = "test"

func TestLane(t *testing.T) {
	tl := NewTestingLane(context.Background())

	lid := tl.LaneId()
	if len(lid) != 36 {
		t.Errorf("wrong lane id length %d", len(lid))
	}

	ctx := context.WithValue(tl, test_str, "pass")

	events := []*laneEvent{}
	tl.Trace("test", "of", "trace")
	events = append(events, &laneEvent{Level: "TRACE", Message: "test of trace"})
	tl.Tracef("testing %d", 123)
	events = append(events, &laneEvent{Level: "TRACE", Message: "testing 123"})

	tl.Debug("test", "of", "debug")
	events = append(events, &laneEvent{Level: "DEBUG", Message: "test of debug"})
	tl.Debugf("testing %d", 456)
	events = append(events, &laneEvent{Level: "DEBUG", Message: "testing 456"})

	tl.Info("test", "of", "info")
	events = append(events, &laneEvent{Level: "INFO", Message: "test of info"})
	tl.Infof("testing %d", 789)
	events = append(events, &laneEvent{Level: "INFO", Message: "testing 789"})

	tl.Warn("test", "of", "warn")
	events = append(events, &laneEvent{Level: "WARN", Message: "test of warn"})
	tl.Warnf("testing %d", 1011)
	events = append(events, &laneEvent{Level: "WARN", Message: "testing 1011"})

	tl.Error("test", "of", "error")
	events = append(events, &laneEvent{Level: "ERROR", Message: "test of error"})
	tl.Errorf("testing %d", 1213)
	events = append(events, &laneEvent{Level: "ERROR", Message: "testing 1213"})

	if !tl.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}

	if ctx.Value(test_str) != string("pass") {
		t.Errorf("Context is not working")
	}
}

func TestLaneSetLevel(t *testing.T) {
	tl := NewTestingLane(context.Background())

	level := tl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	level = tl.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level was not fatal")
	}

	level = tl.SetLogLevel(LogLevelDebug)
	if level != LogLevelDebug {
		t.Error("Log level was not debug")
	}
}

func TestLaneInheritLevel(t *testing.T) {
	tl := NewTestingLane(context.Background())

	level := tl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	tl2 := tl.Derive()

	level = tl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}
}

func TestLaneWithCancel(t *testing.T) {
	tl := NewTestingLane(context.Background())

	level := tl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	tl2, cancel := tl.DeriveWithCancel()

	isDone := make(chan struct{})

	go func() {
		<-tl2.Done()
		isDone <- struct{}{}
	}()

	level = tl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone
}

func TestLaneWithTimeoutCancel(t *testing.T) {
	tl := NewTestingLane(context.Background())

	level := tl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	tl2, cancel := tl.DeriveWithTimeout(time.Hour)

	isDone := make(chan struct{})

	start := time.Now()
	go func() {
		<-tl2.Done()
		isDone <- struct{}{}
	}()

	level = tl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestLaneWithTimeoutExpire(t *testing.T) {
	tl := NewTestingLane(context.Background())

	level := tl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	tl2, _ := tl.DeriveWithTimeout(time.Millisecond)

	isDone := make(chan struct{})

	start := time.Now()
	go func() {
		<-tl2.Done()
		isDone <- struct{}{}
	}()

	level = tl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestLaneWithDeadlineCancel(t *testing.T) {
	tl := NewTestingLane(context.Background())

	level := tl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	start := time.Now()
	tl2, cancel := tl.DeriveWithDeadline(start.Add(time.Minute))

	isDone := make(chan struct{})

	go func() {
		<-tl2.Done()
		isDone <- struct{}{}
	}()

	level = tl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestLaneWithDeadlineExpire(t *testing.T) {
	tl := NewTestingLane(context.Background())

	level := tl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	start := time.Now()
	tl2, _ := tl.DeriveWithDeadline(start.Add(time.Millisecond * 10))

	isDone := make(chan struct{})

	go func() {
		<-tl2.Done()
		isDone <- struct{}{}
	}()

	level = tl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestLaneVerifyText(t *testing.T) {
	tl := NewTestingLane(context.Background())

	tl.Trace("test", "of", "trace")
	tl.Tracef("testing %d", 123)

	tl.Debug("test", "of", "debug")
	tl.Debugf("testing %d", 456)

	tl.Info("test", "of", "info")
	tl.Infof("testing %d", 789)

	tl.Warn("test", "of", "warn")
	tl.Warnf("testing %d", 1011)

	tl.Error("test", "of", "error")
	tl.Errorf("testing %d", 1213)

	expected := `TRACE	test of trace
TRACE	testing 123
DEBUG	test of debug
DEBUG	testing 456
INFO	test of info
INFO	testing 789
WARN	test of warn
WARN	testing 1011
ERROR	test of error
ERROR	testing 1213`

	if !tl.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}
}

func TestLaneVerifyTextTrace(t *testing.T) {
	tl := NewTestingLane(context.Background())

	tl.SetLogLevel(LogLevelDebug)

	tl.Trace("test", "of", "trace")
	tl.Tracef("testing %d", 123)

	tl.Debug("test", "of", "debug")
	tl.Debugf("testing %d", 456)

	tl.Info("test", "of", "info")
	tl.Infof("testing %d", 789)

	tl.Warn("test", "of", "warn")
	tl.Warnf("testing %d", 1011)

	tl.Error("test", "of", "error")
	tl.Errorf("testing %d", 1213)

	expected := `DEBUG	test of debug
DEBUG	testing 456
INFO	test of info
INFO	testing 789
WARN	test of warn
WARN	testing 1011
ERROR	test of error
ERROR	testing 1213`

	if !tl.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}
}

func TestLaneVerifyTextDebug(t *testing.T) {
	tl := NewTestingLane(context.Background())

	tl.SetLogLevel(LogLevelInfo)

	tl.Trace("test", "of", "trace")
	tl.Tracef("testing %d", 123)

	tl.Debug("test", "of", "debug")
	tl.Debugf("testing %d", 456)

	tl.Info("test", "of", "info")
	tl.Infof("testing %d", 789)

	tl.Warn("test", "of", "warn")
	tl.Warnf("testing %d", 1011)

	tl.Error("test", "of", "error")
	tl.Errorf("testing %d", 1213)

	expected := `INFO	test of info
INFO	testing 789
WARN	test of warn
WARN	testing 1011
ERROR	test of error
ERROR	testing 1213`

	if !tl.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}
}

func TestLaneVerifyTextInfo(t *testing.T) {
	tl := NewTestingLane(context.Background())

	tl.SetLogLevel(LogLevelWarn)

	tl.Trace("test", "of", "trace")
	tl.Tracef("testing %d", 123)

	tl.Debug("test", "of", "debug")
	tl.Debugf("testing %d", 456)

	tl.Info("test", "of", "info")
	tl.Infof("testing %d", 789)

	tl.Warn("test", "of", "warn")
	tl.Warnf("testing %d", 1011)

	tl.Error("test", "of", "error")
	tl.Errorf("testing %d", 1213)

	expected := `WARN	test of warn
WARN	testing 1011
ERROR	test of error
ERROR	testing 1213`

	if !tl.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}
}

func TestLaneVerifyTextWarn(t *testing.T) {
	tl := NewTestingLane(context.Background())

	tl.SetLogLevel(LogLevelError)

	tl.Trace("test", "of", "trace")
	tl.Tracef("testing %d", 123)

	tl.Debug("test", "of", "debug")
	tl.Debugf("testing %d", 456)

	tl.Info("test", "of", "info")
	tl.Infof("testing %d", 789)

	tl.Warn("test", "of", "warn")
	tl.Warnf("testing %d", 1011)

	tl.Error("test", "of", "error")
	tl.Errorf("testing %d", 1213)

	expected := `ERROR	test of error
ERROR	testing 1213`

	if !tl.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}
}

func TestLaneVerifyTextError(t *testing.T) {
	tl := NewTestingLane(context.Background())

	tl.SetLogLevel(LogLevelFatal)

	tl.Trace("test", "of", "trace")
	tl.Tracef("testing %d", 123)

	tl.Debug("test", "of", "debug")
	tl.Debugf("testing %d", 456)

	tl.Info("test", "of", "info")
	tl.Infof("testing %d", 789)

	tl.Warn("test", "of", "warn")
	tl.Warnf("testing %d", 1011)

	tl.Error("test", "of", "error")
	tl.Errorf("testing %d", 1213)

	expected := ""

	if !tl.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}
}

func TestLaneVerifyCancel(t *testing.T) {
	tl := NewTestingLane(context.Background())
	l, _ := tl.DeriveWithCancel()

	l.Trace("test of trace")

	expected := "TRACE\ttest of trace"

	tl2, ok := l.(TestingLane)
	if !ok {
		t.Fatal("lane not a testing lane")
	}

	if !tl2.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl2.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}

	if tl.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}

func TestLaneVerifyTimeout(t *testing.T) {
	tl := NewTestingLane(context.Background())
	l, _ := tl.DeriveWithTimeout(time.Hour)

	l.Trace("test of trace")

	expected := "TRACE\ttest of trace"

	tl2, ok := l.(TestingLane)
	if !ok {
		t.Fatal("lane not a testing lane")
	}

	if !tl2.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl2.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}

	if tl.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}

func TestLaneVerifyDeadline(t *testing.T) {
	tl := NewTestingLane(context.Background())
	l, _ := tl.DeriveWithDeadline(time.Now().Add(time.Hour))

	l.Trace("test of trace")

	expected := "TRACE\ttest of trace"

	tl2, ok := l.(TestingLane)
	if !ok {
		t.Fatal("lane not a testing lane")
	}

	if !tl2.VerifyEventText(expected) {
		t.Errorf("Test events don't match")
	}

	if tl2.EventsToString() != expected {
		t.Errorf("Test event string doesn't match")
	}

	if tl.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}

func TestLaneWrappedLogger(t *testing.T) {
	tl := NewTestingLane(context.Background())

	tl.Logger().Println("this is a test")

	if !tl.VerifyEventText("INFO\tthis is a test") {
		t.Errorf("Test events don't match")
	}
}

func TestLaneDerived(t *testing.T) {
	ptl := NewTestingLane(context.Background())
	tl := ptl.Derive().(TestingLane)

	ptl.Logger().Println("this is the parent")
	tl.Logger().Println("this is the child")

	if !ptl.VerifyEventText("INFO\tthis is the parent") {
		t.Errorf("Test events don't match")
	}

	if !tl.VerifyEventText("INFO\tthis is the child") {
		t.Errorf("Test events don't match")
	}
}

func TestLogLane(t *testing.T) {
	ll := NewLogLane(context.Background())

	lid := ll.LaneId()
	if len(lid) != 10 {
		t.Errorf("wrong lane id length %d", len(lid))
	}

	ctx := context.WithValue(ll, test_str, "pass")

	ll.Trace("test", "of", "trace")
	ll.Tracef("testing %d", 123)

	ll.Debug("test", "of", "debug")
	ll.Debugf("testing %d", 456)

	ll.Info("test", "of", "info")
	ll.Infof("testing %d", 789)

	ll.Warn("test", "of", "warn")
	ll.Warnf("testing %d", 1011)

	ll.Error("test", "of", "error")
	ll.Errorf("testing %d", 1213)

	if ctx.Value(test_str).(string) != "pass" {
		t.Errorf("Context is not working")
	}
}

func TestLogLaneSetLevel(t *testing.T) {
	ll := NewLogLane(context.Background())

	level := ll.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	level = ll.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level was not fatal")
	}

	level = ll.SetLogLevel(LogLevelDebug)
	if level != LogLevelDebug {
		t.Error("Log level was not debug")
	}
}

func TestLogLaneInheritLevel(t *testing.T) {
	ll := NewTestingLane(context.Background())

	level := ll.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	ll2 := ll.Derive()

	level = ll2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}
}

func TestLogLaneWithCancel(t *testing.T) {
	ll := NewLogLane(context.Background())

	level := ll.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	ll2, cancel := ll.DeriveWithCancel()

	isDone := make(chan struct{})

	go func() {
		<-ll2.Done()
		isDone <- struct{}{}
	}()

	level = ll2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone
}

func TestLogLaneWithTimeoutCancel(t *testing.T) {
	ll := NewLogLane(context.Background())

	level := ll.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	ll2, cancel := ll.DeriveWithTimeout(time.Hour)

	isDone := make(chan struct{})

	start := time.Now()
	go func() {
		<-ll2.Done()
		isDone <- struct{}{}
	}()

	level = ll2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestLogLaneWithTimeoutExpire(t *testing.T) {
	ll := NewLogLane(context.Background())

	level := ll.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	ll2, _ := ll.DeriveWithTimeout(time.Millisecond)

	isDone := make(chan struct{})

	start := time.Now()
	go func() {
		<-ll2.Done()
		isDone <- struct{}{}
	}()

	level = ll2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestLogLaneWithDeadlineCancel(t *testing.T) {
	ll := NewLogLane(context.Background())

	level := ll.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	start := time.Now()
	ll2, cancel := ll.DeriveWithDeadline(start.Add(time.Minute))

	isDone := make(chan struct{})

	go func() {
		<-ll2.Done()
		isDone <- struct{}{}
	}()

	level = ll2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestLogLaneWithDeadlineExpire(t *testing.T) {
	ll := NewLogLane(context.Background())

	level := ll.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	start := time.Now()
	ll2, _ := ll.DeriveWithDeadline(start.Add(time.Millisecond * 10))

	isDone := make(chan struct{})

	go func() {
		<-ll2.Done()
		isDone <- struct{}{}
	}()

	level = ll2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func verifyLogLaneEvents(t *testing.T, ll Lane, expected string, buf bytes.Buffer) {
	v := ll.Value(log_lane_id)
	if v == nil {
		t.Fatal("missing lane id in context")
	}

	guid := v.(string)
	expected = strings.ReplaceAll(expected, "GUID", guid)

	if expected == "" {
		if buf.Len() != 0 {
			t.Fatal("did not get expected empty log")
		}
	} else {
		expectedLines := strings.Split(expected, "\n")
		actualLines := strings.Split(strings.TrimSpace(buf.String()), "\n")

		if len(expectedLines) != len(actualLines) {
			t.Fatal("did not get expected number of log lines")
		}

		for i, actualLine := range actualLines {
			expectedLine := expectedLines[i]
			if len(expectedLine) < 21 {
				t.Errorf("expected log line is missing the timestamp: %s", expectedLine)
			}
			datePart := actualLine[:20]
			textPart := actualLine[20:]
			if textPart != expectedLine {
				t.Errorf("log events don't match:\n '%s' vs expected\n '%s'", textPart, expectedLine)
			}
			_, err := time.Parse("2006/01/02 15:04:05", strings.TrimSpace(datePart))
			if err != nil {
				t.Errorf("can't parse log timestamp %s", datePart)
			}
		}
	}
}

func TestLogLaneVerifyText(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())

	ll.Trace("test", "of", "trace")
	ll.Tracef("testing %d", 123)

	ll.Debug("test", "of", "debug")
	ll.Debugf("testing %d", 456)

	ll.Info("test", "of", "info")
	ll.Infof("testing %d", 789)

	ll.Warn("test", "of", "warn")
	ll.Warnf("testing %d", 1011)

	ll.Error("test", "of", "error")
	ll.Errorf("testing %d", 1213)

	expected := `TRACE {GUID} test of trace
TRACE {GUID} testing 123
DEBUG {GUID} test of debug
DEBUG {GUID} testing 456
INFO {GUID} test of info
INFO {GUID} testing 789
WARN {GUID} test of warn
WARN {GUID} testing 1011
ERROR {GUID} test of error
ERROR {GUID} testing 1213`

	verifyLogLaneEvents(t, ll, expected, buf)
}

func TestLogLaneVerifyTextCrLf(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLaneWithCR(context.Background())

	ll.Trace("test", "of", "trace")
	ll.Tracef("testing %d", 123)

	expected := "{DATE1} TRACE {GUID} test of trace\r\n{DATE2} TRACE {GUID} testing 123\r\n"

	text := buf.String()

	r := regexp.MustCompile(`(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)
	matches := r.FindStringSubmatch(text)
	if len(matches) != 2 {
		t.Fatal("date")
	}
	date1 := matches[0]
	expected = strings.ReplaceAll(expected, "{DATE1}", date1)
	date2 := matches[1]
	expected = strings.ReplaceAll(expected, "{DATE2}", date2)

	r = regexp.MustCompile(`(\{[a-f0-9]{10}\})`)
	matches = r.FindStringSubmatch(text)
	if len(matches) != 2 {
		t.Fatal("guid")
	}
	guid := matches[0]
	expected = strings.ReplaceAll(expected, "{GUID}", guid)

	if !bytes.Equal(buf.Bytes(), []byte(expected)) {
		t.Error("mismatch")
	}
}

func TestLogLaneVerifyTextFilterTrace(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())
	ll.SetLogLevel(LogLevelDebug)

	ll.Trace("test", "of", "trace")
	ll.Tracef("testing %d", 123)

	ll.Debug("test", "of", "debug")
	ll.Debugf("testing %d", 456)

	ll.Info("test", "of", "info")
	ll.Infof("testing %d", 789)

	ll.Warn("test", "of", "warn")
	ll.Warnf("testing %d", 1011)

	ll.Error("test", "of", "error")
	ll.Errorf("testing %d", 1213)

	expected := `DEBUG {GUID} test of debug
DEBUG {GUID} testing 456
INFO {GUID} test of info
INFO {GUID} testing 789
WARN {GUID} test of warn
WARN {GUID} testing 1011
ERROR {GUID} test of error
ERROR {GUID} testing 1213`

	verifyLogLaneEvents(t, ll, expected, buf)
}

func TestLogLaneVerifyTextFilterDebug(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())
	ll.SetLogLevel(LogLevelInfo)

	ll.Trace("test", "of", "trace")
	ll.Tracef("testing %d", 123)

	ll.Debug("test", "of", "debug")
	ll.Debugf("testing %d", 456)

	ll.Info("test", "of", "info")
	ll.Infof("testing %d", 789)

	ll.Warn("test", "of", "warn")
	ll.Warnf("testing %d", 1011)

	ll.Error("test", "of", "error")
	ll.Errorf("testing %d", 1213)

	expected := `INFO {GUID} test of info
INFO {GUID} testing 789
WARN {GUID} test of warn
WARN {GUID} testing 1011
ERROR {GUID} test of error
ERROR {GUID} testing 1213`

	verifyLogLaneEvents(t, ll, expected, buf)
}

func TestLogLaneVerifyTextFilterInfo(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())
	ll.SetLogLevel(LogLevelWarn)

	ll.Trace("test", "of", "trace")
	ll.Tracef("testing %d", 123)

	ll.Debug("test", "of", "debug")
	ll.Debugf("testing %d", 456)

	ll.Info("test", "of", "info")
	ll.Infof("testing %d", 789)

	ll.Warn("test", "of", "warn")
	ll.Warnf("testing %d", 1011)

	ll.Error("test", "of", "error")
	ll.Errorf("testing %d", 1213)

	expected := `WARN {GUID} test of warn
WARN {GUID} testing 1011
ERROR {GUID} test of error
ERROR {GUID} testing 1213`

	verifyLogLaneEvents(t, ll, expected, buf)
}

func TestLogLaneVerifyTextFilterWarn(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())
	ll.SetLogLevel(LogLevelError)

	ll.Trace("test", "of", "trace")
	ll.Tracef("testing %d", 123)

	ll.Debug("test", "of", "debug")
	ll.Debugf("testing %d", 456)

	ll.Info("test", "of", "info")
	ll.Infof("testing %d", 789)

	ll.Warn("test", "of", "warn")
	ll.Warnf("testing %d", 1011)

	ll.Error("test", "of", "error")
	ll.Errorf("testing %d", 1213)

	expected := `ERROR {GUID} test of error
ERROR {GUID} testing 1213`

	verifyLogLaneEvents(t, ll, expected, buf)
}

func TestLogLaneVerifyTextFilterFatal(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())
	ll.SetLogLevel(LogLevelFatal)

	ll.Trace("test", "of", "trace")
	ll.Tracef("testing %d", 123)

	ll.Debug("test", "of", "debug")
	ll.Debugf("testing %d", 456)

	ll.Info("test", "of", "info")
	ll.Infof("testing %d", 789)

	ll.Warn("test", "of", "warn")
	ll.Warnf("testing %d", 1011)

	ll.Error("test", "of", "error")
	ll.Errorf("testing %d", 1213)

	expected := ""

	verifyLogLaneEvents(t, ll, expected, buf)
}

func TestLogLaneVerifyCancel(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())
	l, _ := ll.DeriveWithCancel()

	l.Trace("test of trace")

	expected := "TRACE {GUID} test of trace"

	ll2, ok := l.(*logLane)
	if !ok {
		t.Fatal("lane not a log lane")
	}

	verifyLogLaneEvents(t, ll2, expected, buf)

	if ll.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}

func TestLogLaneVerifyTimeout(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())
	l, _ := ll.DeriveWithTimeout(time.Hour)

	l.Trace("test of trace")

	expected := "TRACE {GUID} test of trace"

	ll2, ok := l.(*logLane)
	if !ok {
		t.Fatal("lane not a log lane")
	}

	verifyLogLaneEvents(t, ll2, expected, buf)

	if ll.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}

func TestLogLaneVerifyDeadline(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())
	l, _ := ll.DeriveWithDeadline(time.Now().Add(time.Hour))

	l.Trace("test of trace")

	expected := "TRACE {GUID} test of trace"

	ll2, ok := l.(*logLane)
	if !ok {
		t.Fatal("lane not a log lane")
	}

	verifyLogLaneEvents(t, ll2, expected, buf)

	if ll.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}

func TestLogLaneWrappedLogger(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll := NewLogLane(context.Background())

	ll.Logger().Println("this is a test")

	verifyLogLaneEvents(t, ll, "INFO {GUID} this is a test", buf)
}

func TestLogLaneDerivation(t *testing.T) {
	pll := NewLogLane(context.Background())
	ll := pll.Derive()

	var buf1 bytes.Buffer
	log.SetOutput(&buf1)
	defer func() { log.SetOutput(os.Stderr) }()
	ll.Logger().Println("this is a test")

	var buf2 bytes.Buffer
	log.SetOutput(&buf2)
	pll.Logger().Println("this is the parent")

	verifyLogLaneEvents(t, ll, "INFO {GUID} this is a test", buf1)
	verifyLogLaneEvents(t, pll, "INFO {GUID} this is the parent", buf2)
}

func TestNullLane(t *testing.T) {
	nl := NewNullLane(context.Background())

	lid := nl.LaneId()
	if len(lid) != 36 {
		t.Errorf("wrong lane id length %d", len(lid))
	}

	nl.Trace("this is ignored")
	nl.Tracef("this is %s", "ignored")
	nl.Debug("this is ignored")
	nl.Debugf("this is %s", "ignored")
	nl.Info("this is ignored")
	nl.Infof("this is %s", "ignored")
	nl.Warn("this is ignored")
	nl.Warnf("this is %s", "ignored")
	nl.Error("this is ignored")
	nl.Errorf("this is %s", "ignored")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()
	nl.Logger().Println("this is a test")

	if buf.Len() != 0 {
		t.Errorf("unexpected data in logger output")
	}
}

func TestNullLaneSetLevel(t *testing.T) {
	nl := NewNullLane(context.Background())

	level := nl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	level = nl.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level was not fatal")
	}

	level = nl.SetLogLevel(LogLevelDebug)
	if level != LogLevelDebug {
		t.Error("Log level was not debug")
	}
}

func TestNullLaneInheritLevel(t *testing.T) {
	nl := NewNullLane(context.Background())

	level := nl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	nl2 := nl.Derive()

	level = nl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}
}

func TestNullLaneWithCancel(t *testing.T) {
	nl := NewNullLane(context.Background())

	level := nl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	nl2, cancel := nl.DeriveWithCancel()

	isDone := make(chan struct{})

	go func() {
		<-nl2.Done()
		isDone <- struct{}{}
	}()

	level = nl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone
}

func TestNullLaneWithTimeoutCancel(t *testing.T) {
	nl := NewNullLane(context.Background())

	level := nl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	nl2, cancel := nl.DeriveWithTimeout(time.Hour)

	isDone := make(chan struct{})

	start := time.Now()
	go func() {
		<-nl2.Done()
		isDone <- struct{}{}
	}()

	level = nl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestNullLaneWithTimeoutExpire(t *testing.T) {
	nl := NewNullLane(context.Background())

	level := nl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	nl2, _ := nl.DeriveWithTimeout(time.Millisecond)

	isDone := make(chan struct{})

	start := time.Now()
	go func() {
		<-nl2.Done()
		isDone <- struct{}{}
	}()

	level = nl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestNullLaneWithDeadlineCancel(t *testing.T) {
	nl := NewNullLane(context.Background())

	level := nl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	start := time.Now()
	nl2, cancel := nl.DeriveWithDeadline(start.Add(time.Minute))

	isDone := make(chan struct{})

	go func() {
		<-nl2.Done()
		isDone <- struct{}{}
	}()

	level = nl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	time.Sleep(time.Millisecond)
	cancel()

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestNullLaneWithDeadlineExpire(t *testing.T) {
	nl := NewNullLane(context.Background())

	level := nl.SetLogLevel(LogLevelFatal)
	if level != LogLevelTrace {
		t.Error("Log level not initially trace")
	}

	start := time.Now()
	nl2, _ := nl.DeriveWithDeadline(start.Add(time.Millisecond * 10))

	isDone := make(chan struct{})

	go func() {
		<-nl2.Done()
		isDone <- struct{}{}
	}()

	level = nl2.SetLogLevel(LogLevelDebug)
	if level != LogLevelFatal {
		t.Error("Log level 2 was not fatal")
	}

	<-isDone

	delta := time.Since(start)
	if delta.Milliseconds() > 60 {
		t.Error("Timeout too long")
	}
}

func TestNullLaneDerivation(t *testing.T) {
	pll := NewNullLane(context.Background())
	ll := pll.Derive()

	var buf1 bytes.Buffer
	log.SetOutput(&buf1)
	defer func() { log.SetOutput(os.Stderr) }()
	ll.Logger().Println("this is a test")

	var buf2 bytes.Buffer
	log.SetOutput(&buf2)
	pll.Logger().Println("this is the parent")

	if buf1.Len() != 0 || buf2.Len() != 0 {
		t.Errorf("unexpected data in logger output")
	}
}

func TestNullLaneVerifyCancel(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	nl := NewNullLane(context.Background())
	l, _ := nl.DeriveWithCancel()

	l.Trace("test of trace")

	_, ok := l.(*nullLane)
	if !ok {
		t.Fatal("lane not a null lane")
	}

	if nl.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}

func TestNullLaneVerifyTimeout(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	nl := NewNullLane(context.Background())
	l, _ := nl.DeriveWithTimeout(time.Hour)

	l.Trace("test of trace")

	_, ok := l.(*nullLane)
	if !ok {
		t.Fatal("lane not a null lane")
	}

	if nl.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}

func TestNullLaneVerifyDeadline(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	nl := NewNullLane(context.Background())
	l, _ := nl.DeriveWithDeadline(time.Now().Add(time.Hour))

	l.Trace("test of trace")

	_, ok := l.(*nullLane)
	if !ok {
		t.Fatal("lane not a null lane")
	}

	if nl.LaneId() == l.LaneId() {
		t.Errorf("Lane IDs match")
	}

	if len(l.LaneId()) < 6 {
		t.Errorf("insufficient lane id")
	}
}
