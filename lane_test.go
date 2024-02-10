package lane

import (
	"bytes"
	"context"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type testKeyType string
type testValueType string

const kTestStr testKeyType = "test"
const kTestBase testKeyType = "base"
const kTestReplaced testValueType = "replaced"

func TestLane(t *testing.T) {
	tl := NewTestingLane(context.Background())

	lid := tl.LaneId()
	if len(lid) != 36 {
		t.Errorf("wrong lane id length %d", len(lid))
	}

	ctx := context.WithValue(tl, kTestStr, "pass")

	events := []*LaneEvent{}
	events2 := []*LaneEvent{}
	tl.Trace("test", "of", "trace")
	events = append(events, &LaneEvent{Level: "TRACE", Message: "test of trace"})
	tl.Tracef("testing %d", 123)
	events = append(events, &LaneEvent{Level: "TRACE", Message: "testing 123"})

	tl.Debug("test", "of", "debug")
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "test of debug"})
	events2 = append(events2, &LaneEvent{Level: "DEBUG", Message: "test of debug"})
	tl.Debugf("testing %d", 456)
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "testing 456"})

	tl.Info("test", "of", "info")
	events = append(events, &LaneEvent{Level: "INFO", Message: "test of info"})
	tl.Infof("testing %d", 789)
	events = append(events, &LaneEvent{Level: "INFO", Message: "testing 789"})
	events2 = append(events2, &LaneEvent{Level: "INFO", Message: "testing 789"})

	tl.Warn("test", "of", "warn")
	events = append(events, &LaneEvent{Level: "WARN", Message: "test of warn"})
	tl.Warnf("testing %d", 1011)
	events = append(events, &LaneEvent{Level: "WARN", Message: "testing 1011"})

	tl.Error("test", "of", "error")
	events = append(events, &LaneEvent{Level: "ERROR", Message: "test of error"})
	tl.Errorf("testing %d", 1213)
	events = append(events, &LaneEvent{Level: "ERROR", Message: "testing 1213"})
	events2 = append(events2, &LaneEvent{Level: "ERROR", Message: "testing 1213"})

	if !tl.VerifyEvents(events) || tl.VerifyEvents(events2) {
		t.Errorf("Test events don't match")
	}

	if !tl.FindEvents(events) || !tl.FindEvents(events2) {
		t.Errorf("Test events don't match 2")
	}

	if ctx.Value(kTestStr) != string("pass") {
		t.Errorf("Context is not working")
	}

	// unset metadata
	if tl.GetMetadata("key") != "" {
		t.Error("test lane must provide empty value when metadata is not set")
	}

	// setting metadata via the generic interface is visible in a testing lane
	l := Lane(tl)
	l.SetMetadata("key", "stored")

	if tl.GetMetadata("key") != "stored" {
		t.Error("test lane must provide access to metadata")
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

func TestLaneReplaceContext(t *testing.T) {
	c1 := context.WithValue(context.Background(), kTestBase, kTestBase)
	tl := NewTestingLane(c1)

	c2 := context.WithValue(context.Background(), kTestBase, kTestReplaced)
	tl2 := tl.DeriveReplaceContext(c2)

	if tl2.Value(kTestBase) != kTestReplaced {
		t.Error("Base not replaced")
	}

	tl3 := tl2.Derive()
	if tl3.Value(kTestBase) != kTestReplaced {
		t.Error("Derived incorrect")
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

func TestLaneFindText(t *testing.T) {
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

	expected1 := `TRACE	test of trace
DEBUG	test of debug
INFO	test of info
INFO	testing 789
WARN	testing 1011
ERROR	testing 1213`

	if tl.VerifyEventText(expected1) {
		t.Errorf("Test events don't match")
	}

	if !tl.FindEventText(expected1) {
		t.Errorf("Test events don't match")
	}

	// out of order log messages will not match
	expected2 := `TRACE	test of trace
INFO	test of info
DEBUG	test of debug
INFO	testing 789
WARN	testing 1011
ERROR	testing 1213`

	if tl.FindEventText(expected2) {
		t.Errorf("Test events don't match")
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

func TestLaneDerivedCaptureChild(t *testing.T) {
	ptl := NewTestingLane(context.Background())
	tl := ptl.Derive().(TestingLane)
	prior := ptl.WantDescendantEvents(true)
	if prior {
		t.Error("unexpected prior value")
	}
	prior = ptl.WantDescendantEvents(true)
	if !prior {
		t.Error("unexpected prior value")
	}

	ptl.Logger().Println("this is the parent")
	tl.Logger().Println("this is the child")

	if !ptl.VerifyEventText("INFO\tthis is the parent\nINFO\tthis is the child") {
		t.Errorf("Test events don't match")
	}

	if !tl.VerifyEventText("INFO\tthis is the child") {
		t.Errorf("Test events don't match")
	}
}

func TestLaneDerivedCaptureGrandchild(t *testing.T) {
	gptl := NewTestingLane(context.Background())
	ptl := gptl.Derive().(TestingLane)
	tl := ptl.Derive().(TestingLane)

	gptl.WantDescendantEvents(true)

	gptl.Logger().Println("this is the grandparent")
	ptl.Logger().Println("this is the parent")
	tl.Logger().Println("this is the child")

	if !gptl.VerifyEventText("INFO\tthis is the grandparent\nINFO\tthis is the parent\nINFO\tthis is the child") {
		t.Errorf("Test events don't match")
	}

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

	ctx := context.WithValue(ll, kTestStr, "pass")

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

	if ctx.Value(kTestStr).(string) != "pass" {
		t.Errorf("Context is not working")
	}

	// setting metadata is harmless
	l := Lane(ll)
	l.SetMetadata("key", "ignored")
}

func TestLogLaneJourneyId(t *testing.T) {
	ll := NewLogLane(context.Background())
	id := uuid.New().String()
	id = id[len(id)-10:]
	ll.SetJourneyId(id)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll.Info("test", "of", "info")

	capture := buf.String()
	if !strings.Contains(capture, id) {
		t.Error("did not find outer correlation id")
	}
	if !strings.Contains(capture, ll.LaneId()) {
		t.Error("did not find lane correlation id")
	}
}

func TestLogLaneJourneyIdDerived(t *testing.T) {
	ll := NewLogLane(context.Background())
	id := uuid.New().String()
	id = id[len(id)-10:]
	ll.SetJourneyId(id)

	ll2 := ll.Derive()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll2.Info("test", "of", "info")

	capture := buf.String()
	if !strings.Contains(capture, id) {
		t.Error("did not find outer correlation id")
	}
	if strings.Contains(capture, ll.LaneId()) {
		t.Error("found unexpected correlation id")
	}
	if !strings.Contains(capture, ll2.LaneId()) {
		t.Error("did not find lane correlation id")
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
	v := ll.Value(LogLaneIdKey)
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
				if !strings.HasSuffix(expectedLine, "{ANY}") || !strings.HasPrefix(textPart, expectedLine[:len(expectedLine)-5]) {
					t.Errorf("log events don't match:\n '%s' vs expected\n '%s'", textPart, expectedLine)
				}
			}
			_, err := time.Parse("2006/01/02 15:04:05", strings.TrimSpace(datePart))
			if err != nil {
				t.Errorf("can't parse log timestamp %s", datePart)
			}
		}
	}
}

func TestLogLaneReplaceContext(t *testing.T) {
	c1 := context.WithValue(context.Background(), kTestBase, kTestBase)
	ll := NewLogLane(c1)

	c2 := context.WithValue(context.Background(), kTestBase, kTestReplaced)
	ll2 := ll.DeriveReplaceContext(c2)

	if ll2.Value(kTestBase) != kTestReplaced {
		t.Error("Base not replaced")
	}

	ll3 := ll2.Derive()
	if ll3.Value(kTestBase) != kTestReplaced {
		t.Error("Derived incorrect")
	}
}

func TestLogLaneEnableStack(t *testing.T) {
	ll := NewLogLane(context.Background())

	for level := LogLevelTrace; level <= LogLevelFatal; level++ {
		v := ll.EnableStackTrace(level, true)
		if v {
			t.Error("expected false")
		}

		v = ll.EnableStackTrace(level, true)
		if !v {
			t.Error("expected true")
		}
	}

	for level := LogLevelTrace; level <= LogLevelFatal; level++ {
		v := ll.EnableStackTrace(level, false)
		if !v {
			t.Error("expected false")
		}

		v = ll.EnableStackTrace(level, false)
		if v {
			t.Error("expected false")
		}
	}
}

func TestLogLaneEnableStack2(t *testing.T) {
	ll := NewLogLane(context.Background())

	v := ll.EnableStackTrace(LogLevelError, true)
	if v {
		t.Error("expected false")
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll.Error("test", "of", "error")
	ll.Errorf("testing %d", 1213)

	expected := `ERROR {GUID} test of error
STACK {GUID} {ANY}
STACK {GUID} {ANY}
STACK {GUID} {ANY}
STACK {GUID} {ANY}
STACK {GUID} {ANY}
STACK {GUID} {ANY}
ERROR {GUID} testing 1213
STACK {GUID} {ANY}
STACK {GUID} {ANY}
STACK {GUID} {ANY}
STACK {GUID} {ANY}
STACK {GUID} {ANY}
STACK {GUID} {ANY}`

	verifyLogLaneEvents(t, ll, expected, buf)
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

	// setting metadata is harmless
	l := Lane(nl)
	l.SetMetadata("key", "ignored")
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

func TestNullLaneReplaceContext(t *testing.T) {
	c1 := context.WithValue(context.Background(), kTestBase, kTestBase)
	nl := NewNullLane(c1)

	c2 := context.WithValue(context.Background(), kTestBase, kTestReplaced)
	nl2 := nl.DeriveReplaceContext(c2)

	if nl2.Value(kTestBase) != kTestReplaced {
		t.Error("Base not replaced")
	}

	nl3 := nl2.Derive()
	if nl3.Value(kTestBase) != kTestReplaced {
		t.Error("Derived incorrect")
	}
}

func TestNullLaneEnableStack(t *testing.T) {
	nl := NewNullLane(context.Background())

	for level := LogLevelTrace; level <= LogLevelFatal; level++ {
		v := nl.EnableStackTrace(level, true)
		if v {
			t.Error("expected false")
		}

		v = nl.EnableStackTrace(level, true)
		if !v {
			t.Error("expected true")
		}
	}

	for level := LogLevelTrace; level <= LogLevelFatal; level++ {
		v := nl.EnableStackTrace(level, false)
		if !v {
			t.Error("expected false")
		}

		v = nl.EnableStackTrace(level, false)
		if v {
			t.Error("expected false")
		}
	}
}

func TestDiskLane(t *testing.T) {
	os.Remove("test.log")

	dl, err := NewDiskLane(context.Background(), "test.log")
	if err != nil {
		t.Fatal("make test.log")
	}
	if dl == nil {
		t.Fatal("nil disk lane")
	}

	dl.Info("testing 123")

	dl2 := dl.Derive()
	dl.Close()

	dl2.Info("testing 456")
	dl2.Close()

	bytes, err := os.ReadFile("test.log")
	if err != nil {
		t.Fatalf("read test.log: %v", err)
	}

	text := string(bytes)
	if !strings.Contains(text, "testing 123\n") || !strings.Contains(text, "testing 456\n") {
		t.Errorf("incorrect contents of disk log file")
	}

	os.Remove("test.log")
}

func TestDiskLaneBadPath(t *testing.T) {
	_, err := NewDiskLane(context.Background(), "")
	if err == nil {
		t.Fatal("make test.log")
	}
}

func TestLogLanePlainDeriveWithCancel(t *testing.T) {
	ll := NewLogLane(context.Background())
	ll2, _ := ll.DeriveWithCancel()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll2.Info("testing 123")

	capture := buf.String()
	if !strings.HasSuffix(capture, "testing 123\n") {
		t.Error("\\r found in derived lane")
	}
}

func TestLogLaneWithCrDeriveWithCancel(t *testing.T) {
	ll := NewLogLaneWithCR(context.Background())
	ll2, _ := ll.DeriveWithCancel()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll2.Info("testing 123")

	capture := buf.String()
	if !strings.HasSuffix(capture, "testing 123\r\n") {
		t.Error("\\r not found in derived lane")
	}
}

func TestLogLaneWithMicroseconds(t *testing.T) {
	ll := NewLogLane(context.Background())

	ll.Logger().SetFlags(ll.Logger().Flags() | log.Ldate | log.Lmicroseconds)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll.Info("testing 123")

	capture := buf.String()
	match, _ := regexp.MatchString(`\d+/\d+/\d+ \d+:\d+:\d+\.\d+`, capture)
	if !match {
		t.Errorf("did not find microseconds in %s", capture)
	}
}

func TestLogLaneWithMicrosecondsDerive(t *testing.T) {
	ll := NewLogLane(context.Background())

	ll.Logger().SetFlags(ll.Logger().Flags() | log.Ldate | log.Lmicroseconds)
	ll2 := ll.Derive()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll2.Info("testing 123")

	capture := buf.String()
	match, _ := regexp.MatchString(`\d+/\d+/\d+ \d+:\d+:\d+\.\d+`, capture)
	if !match {
		t.Errorf("did not find microseconds in %s", capture)
	}
}

func TestLogLaneWithPrefix(t *testing.T) {
	ll := NewLogLane(context.Background())

	ll.Logger().SetPrefix("myprefix")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll.Info("testing 123")

	capture := buf.String()
	if !strings.Contains(capture, "myprefix") {
		t.Errorf("did not find prefix in %s", capture)
	}
}

func TestLogLaneWithPrefixDerive(t *testing.T) {
	ll := NewLogLane(context.Background())

	ll.Logger().SetPrefix("myprefix")
	ll2 := ll.Derive()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll2.Info("testing 123")

	capture := buf.String()
	if !strings.Contains(capture, "myprefix") {
		t.Errorf("did not find prefix in %s", capture)
	}
}

func TestLogLaneDateTimeDefault(t *testing.T) {
	ll := NewLogLane(context.Background())

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ll.Info("testing 123")

	capture := buf.String()
	match, _ := regexp.MatchString(`\d+/\d+/\d+ \d+:\d+:\d+\ `, capture)
	if !match {
		t.Errorf("did not find date/time in %s", capture)
	}
}

func setTestPanicHandler(l Lane) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	l.SetPanicHandler(func() {
		wg.Done()
		runtime.Goexit()
	})
	return &wg
}

func TestPanicTestLane(t *testing.T) {
	tl := NewTestingLane(context.Background())
	wg := setTestPanicHandler(tl)
	go func() {
		tl.Fatal("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicTestLaneF(t *testing.T) {
	tl := NewTestingLane(context.Background())
	wg := setTestPanicHandler(tl)
	go func() {
		tl.Fatalf("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicTestLaneDerived(t *testing.T) {
	tl := NewTestingLane(context.Background())
	wg := setTestPanicHandler(tl)
	tl2 := tl.Derive()
	go func() {
		tl2.Fatal("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicLogLane(t *testing.T) {
	ll := NewLogLane(context.Background())
	wg := setTestPanicHandler(ll)
	go func() {
		ll.Fatal("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicLogLanef(t *testing.T) {
	ll := NewLogLane(context.Background())
	wg := setTestPanicHandler(ll)
	go func() {
		ll.Fatalf("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicLogLaneDerived(t *testing.T) {
	ll := NewLogLane(context.Background())
	wg := setTestPanicHandler(ll)
	ll2 := ll.Derive()
	go func() {
		ll2.Fatal("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicNullLane(t *testing.T) {
	nl := NewNullLane(context.Background())
	wg := setTestPanicHandler(nl)
	go func() {
		nl.Fatal("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicNullLanef(t *testing.T) {
	nl := NewNullLane(context.Background())
	wg := setTestPanicHandler(nl)
	go func() {
		nl.Fatalf("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicNullLaneDerived(t *testing.T) {
	nl := NewNullLane(context.Background())
	wg := setTestPanicHandler(nl)
	nl2 := nl.Derive()
	go func() {
		nl2.Fatal("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicDiskLane(t *testing.T) {
	dl, err := NewDiskLane(context.Background(), "test.log")
	if err != nil {
		t.Fatal("make test.log")
	}
	wg := setTestPanicHandler(dl)
	go func() {
		dl.Fatal("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicDiskLanef(t *testing.T) {
	dl, err := NewDiskLane(context.Background(), "test.log")
	if err != nil {
		t.Fatal("make test.log")
	}
	wg := setTestPanicHandler(dl)
	go func() {
		dl.Fatalf("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}

func TestPanicDiskLaneDerived(t *testing.T) {
	dl, err := NewDiskLane(context.Background(), "test.log")
	if err != nil {
		t.Fatal("make test.log")
	}
	wg := setTestPanicHandler(dl)
	dl2 := dl.Derive()
	go func() {
		dl2.Fatal("stop me")
		panic("unreachable")
	}()
	wg.Wait()
}
