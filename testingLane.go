package lane

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type (
	laneEvent struct {
		Id      string
		Level   string
		Message string
	}

	testingLane struct {
		mu sync.Mutex
		context.Context
		Events               []*laneEvent
		tlog                 *log.Logger
		level                LaneLogLevel
		stackTrace           []atomic.Bool
		tees                 []Lane
		parent               *testingLane
		wantDescendantEvents bool
	}

	testingLaneId string

	testingLogWriter struct {
		tl *testingLane
	}

	TestingLane interface {
		Lane
		EventsToString() string
		VerifyEvents(eventList []*laneEvent) (match bool)
		VerifyEventText(eventText string) (match bool)
		WantDescendantEvents(wanted bool) (prior bool)
	}
)

const testing_lane_id testingLaneId = "testing_lane"

func NewTestingLane(ctx context.Context) TestingLane {
	return deriveTestingLane(ctx, nil, []Lane{})
}

func deriveTestingLane(ctx context.Context, parent *testingLane, tees []Lane) TestingLane {
	tl := testingLane{
		stackTrace: make([]atomic.Bool, int(LogLevelFatal+1)),
		parent:     parent,
		tees:       tees,
	}

	// make a logging instance that ultimately does logging via the lane
	tlw := testingLogWriter{tl: &tl}
	tl.tlog = log.New(&tlw, "", 0)

	tl.Context = context.WithValue(ctx, testing_lane_id, uuid.New().String())
	return &tl
}

func (tl *testingLane) SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	priorLevel = tl.level
	tl.level = newLevel
	return
}

func (tl *testingLane) VerifyEvents(eventList []*laneEvent) bool {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if len(eventList) != len(tl.Events) {
		return false
	}

	for i := 0; i < len(eventList); i++ {
		e1 := eventList[i]
		e2 := tl.Events[i]

		if e1.Level != e2.Level ||
			e1.Message != e2.Message {
			return false
		}
	}

	return true
}

// eventText specifies a list of events, separated by \n, and each
// line must be in the form of <level>\t<message>.
func (tl *testingLane) VerifyEventText(eventText string) (match bool) {
	eventList := []*laneEvent{}

	if eventText != "" {
		lines := strings.Split(eventText, "\n")
		for _, line := range lines {
			parts := strings.Split(line, "\t")
			if len(parts) != 2 {
				panic("eventText line must have exactly one tab separator")
			}
			eventList = append(eventList, &laneEvent{Level: parts[0], Message: parts[1]})
		}
	}

	return tl.VerifyEvents(eventList)
}

func (tl *testingLane) EventsToString() string {
	var sb strings.Builder

	for _, e := range tl.Events {
		if sb.Len() > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(e.Level)
		sb.WriteRune('\t')
		sb.WriteString(e.Message)
	}

	return sb.String()
}

func (tl *testingLane) WantDescendantEvents(wanted bool) bool {
	tl.mu.Lock()
	prior := tl.wantDescendantEvents
	tl.wantDescendantEvents = wanted
	tl.mu.Unlock()

	return prior
}

func (tl *testingLane) recordLaneEvent(level LaneLogLevel, levelText string, format *string, args ...any) {
	tl.recordLaneEventRecursive(true, level, levelText, format, args...)
}

// Worker that adds the test event to the testing lane, and then passes it up to the parent,
// where the parent decides to capture it as well, and then passes it up to the
// grandparent, and so on.
func (tl *testingLane) recordLaneEventRecursive(originator bool, level LaneLogLevel, levelText string, format *string, args ...any) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if originator || tl.wantDescendantEvents {
		if level >= tl.level {
			le := laneEvent{
				Id:    "global",
				Level: levelText,
			}

			if format == nil {
				le.Message = fmt.Sprintln(args...)          // use Sprintln because it matches log behavior wrt spaces between args
				le.Message = le.Message[:len(le.Message)-1] // remove \n
			} else {
				le.Message = fmt.Sprintf(*format, args...)
			}

			v := tl.Value(testing_lane_id)
			if v != nil {
				le.Id = v.(string)
			}

			tl.Events = append(tl.Events, &le)
		}
	}

	if tl.parent != nil {
		tl.parent.recordLaneEventRecursive(false, level, levelText, format, args...)
	}
}

func (tl *testingLane) tee(logger func(l Lane)) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	for _, t := range tl.tees {
		logger(t)
	}
}

func (tl *testingLane) Trace(args ...any) {
	tl.recordLaneEvent(LogLevelTrace, "TRACE", nil, args...)
	tl.tee(func(l Lane) { l.Trace(args...) })
}

func (tl *testingLane) Tracef(format string, args ...any) {
	tl.recordLaneEvent(LogLevelTrace, "TRACE", &format, args...)
	tl.tee(func(l Lane) { l.Tracef(format, args...) })
}

func (tl *testingLane) Debug(args ...any) {
	tl.recordLaneEvent(LogLevelDebug, "DEBUG", nil, args...)
	tl.tee(func(l Lane) { l.Debug(args...) })
}

func (tl *testingLane) Debugf(format string, args ...any) {
	tl.recordLaneEvent(LogLevelDebug, "DEBUG", &format, args...)
	tl.tee(func(l Lane) { l.Debugf(format, args...) })
}

func (tl *testingLane) Info(args ...any) {
	tl.recordLaneEvent(LogLevelInfo, "INFO", nil, args...)
	tl.tee(func(l Lane) { l.Info(args...) })
}

func (tl *testingLane) Infof(format string, args ...any) {
	tl.recordLaneEvent(LogLevelInfo, "INFO", &format, args...)
	tl.tee(func(l Lane) { l.Infof(format, args...) })
}

func (tl *testingLane) Warn(args ...any) {
	tl.recordLaneEvent(LogLevelWarn, "WARN", nil, args...)
	tl.tee(func(l Lane) { l.Warn(args...) })
}

func (tl *testingLane) Warnf(format string, args ...any) {
	tl.recordLaneEvent(LogLevelWarn, "WARN", &format, args...)
	tl.tee(func(l Lane) { l.Warnf(format, args...) })
}

func (tl *testingLane) Error(args ...any) {
	tl.recordLaneEvent(LogLevelError, "ERROR", nil, args...)
	tl.logStack(LogLevelError)
	tl.tee(func(l Lane) { l.Error(args...) })
}

func (tl *testingLane) Errorf(format string, args ...any) {
	tl.recordLaneEvent(LogLevelError, "ERROR", &format, args...)
	tl.logStack(LogLevelError)
	tl.tee(func(l Lane) { l.Errorf(format, args...) })
}

func (tl *testingLane) PreFatal(args ...any) {
	tl.recordLaneEvent(LogLevelFatal, "FATAL", nil, args...)
	tl.tee(func(l Lane) { l.PreFatal(args...) })
}

func (tl *testingLane) PreFatalf(format string, args ...any) {
	tl.recordLaneEvent(LogLevelFatal, "FATAL", &format, args...)
	tl.tee(func(l Lane) { l.PreFatalf(format, args...) })
}

func (tl *testingLane) Fatal(args ...any) {
	tl.PreFatal(args...)
	panic("fatal error") // test must recover
}

func (tl *testingLane) Fatalf(format string, args ...any) {
	tl.PreFatalf(format, args...)
	panic("fatal error") // test must recover
}

func (tl *testingLane) logStack(level LaneLogLevel) {
	if tl.stackTrace[level].Load() {
		buf := make([]byte, 16384)
		n := runtime.Stack(buf, false)
		format := "%s"
		tl.recordLaneEvent(level, "STACK", &format, string(buf[0:n]))
	}
}

func (tl *testingLane) Logger() *log.Logger {
	return tl.tlog
}

func (tl *testingLane) Close() {
}

func (tl *testingLane) Derive() Lane {
	l := deriveTestingLane(context.WithValue(tl.Context, parent_lane_id, tl.LaneId()), tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l
}

func (tl *testingLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(tl.Context, parent_lane_id, tl.LaneId()))
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadline(context.WithValue(tl.Context, parent_lane_id, tl.LaneId()), deadline)
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeout(context.WithValue(tl.Context, parent_lane_id, tl.LaneId()), duration)
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveReplaceContext(ctx context.Context) Lane {
	l := NewTestingLane(ctx)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l
}

func (tl *testingLane) EnableStackTrace(level LaneLogLevel, enable bool) bool {
	return tl.stackTrace[level].Swap(enable)
}

func (tl *testingLane) LaneId() string {
	return tl.Value(testing_lane_id).(string)
}

func (tl *testingLane) AddTee(l Lane) {
	tl.mu.Lock()
	tl.tees = append(tl.tees, l)
	tl.mu.Unlock()
}

func (tl *testingLane) RemoveTee(l Lane) {
	tl.mu.Lock()
	for i, t := range tl.tees {
		if t.LaneId() == l.LaneId() {
			tl.tees = append(tl.tees[:i], tl.tees[i+1:]...)
			break
		}
	}
	tl.mu.Unlock()
}

func (tlw *testingLogWriter) Write(p []byte) (n int, err error) {
	text := strings.TrimSuffix(string(p), "\n")
	tlw.tl.Info(text)
	return len(p), nil
}
