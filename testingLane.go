package lane

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

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
		Events []*laneEvent
		tlog   *log.Logger
		level  LaneLogLevel
	}

	testingLaneId string

	testingLogWriter struct {
		tl *testingLane
	}

	TestingLane interface {
		Lane
		VerifyEvents(eventList []*laneEvent) (match bool)
		VerifyEventText(eventText string) (match bool)
	}
)

const testing_lane_id testingLaneId = "testing_lane"

func NewTestingLane(ctx context.Context) TestingLane {
	tl := testingLane{}

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

func (tl *testingLane) recordLaneEvent(level LaneLogLevel, levelText string, format *string, args ...any) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

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

func (tl *testingLane) Trace(args ...any) {
	tl.recordLaneEvent(LogLevelTrace, "TRACE", nil, args...)
}

func (tl *testingLane) Tracef(format string, args ...any) {
	tl.recordLaneEvent(LogLevelTrace, "TRACE", &format, args...)
}

func (tl *testingLane) Debug(args ...any) {
	tl.recordLaneEvent(LogLevelDebug, "DEBUG", nil, args...)
}

func (tl *testingLane) Debugf(format string, args ...any) {
	tl.recordLaneEvent(LogLevelDebug, "DEBUG", &format, args...)
}

func (tl *testingLane) Info(args ...any) {
	tl.recordLaneEvent(LogLevelInfo, "INFO", nil, args...)
}

func (tl *testingLane) Infof(format string, args ...any) {
	tl.recordLaneEvent(LogLevelInfo, "INFO", &format, args...)
}

func (tl *testingLane) Warn(args ...any) {
	tl.recordLaneEvent(LogLevelWarn, "WARN", nil, args...)
}

func (tl *testingLane) Warnf(format string, args ...any) {
	tl.recordLaneEvent(LogLevelWarn, "WARN", &format, args...)
}

func (tl *testingLane) Error(args ...any) {
	tl.recordLaneEvent(LogLevelError, "ERROR", nil, args...)
}

func (tl *testingLane) Errorf(format string, args ...any) {
	tl.recordLaneEvent(LogLevelError, "ERROR", &format, args...)
}

func (tl *testingLane) Fatal(args ...any) {
	tl.recordLaneEvent(LogLevelFatal, "FATAL", nil, args...)
	panic("fatal error") // test must recover
}

func (tl *testingLane) Fatalf(format string, args ...any) {
	tl.recordLaneEvent(LogLevelFatal, "FATAL", &format, args...)
	panic("fatal error") // test must recover
}

func (tl *testingLane) Logger() *log.Logger {
	return tl.tlog
}

func (tl *testingLane) Derive() Lane {
	l := NewTestingLane(context.WithValue(tl.Context, parent_lane_id, tl.LaneId()))

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l
}

func (tl *testingLane) LaneId() string {
	return tl.Value(testing_lane_id).(string)
}

func (tlw *testingLogWriter) Write(p []byte) (n int, err error) {
	text := strings.TrimSuffix(string(p), "\n")
	tlw.tl.Info(text)
	return len(p), nil
}
