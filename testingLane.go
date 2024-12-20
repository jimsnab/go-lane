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
)

type (
	LaneEvent struct {
		Id      string
		Level   string
		Message string
	}

	testingLane struct {
		mu sync.Mutex
		context.Context
		MetadataStore
		Events               []*LaneEvent
		tlog                 *log.Logger
		level                LaneLogLevel
		stackTrace           []atomic.Bool
		testingStack         atomic.Bool
		tees                 []Lane
		parent               *testingLane
		wantDescendantEvents bool
		onPanic              Panic
		journeyId            string
		maxLength            atomic.Int32
	}

	testingLaneId string

	testingLogWriter struct {
		tl *testingLane
	}

	TestingLane interface {
		Lane

		// Renders all of the captured log messages into a single string.
		EventsToString() string

		// Checks for log messages to exactly match the specified events.
		VerifyEvents(eventList []*LaneEvent) (match bool)

		// Checks for log messages to match the specified events. Ignores
		// log events that do not match.
		FindEvents(eventList []*LaneEvent) (found bool)

		// Uses a descriptor to create an event list, then calls VerifyEvents.
		// The descriptor is a simple format where log messages are separated
		// by line breaks, and each line is "SEVERITY\tExpected message". The
		// other details that get logged, such as timestamp and correlation ID,
		// are ignored.
		VerifyEventText(eventText string) (match bool)

		// Similar to VerifyEventText, except that lines that do not match
		// are ignored.
		FindEventText(eventText string) (found bool)

		// Controls whether to capture child lane activity (wanted=true) or not.
		WantDescendantEvents(wanted bool) (prior bool)

		// Retrieves metadata
		GetMetadata(key string) string

		// Controls whether stack traces are a single event or an event per
		// call stack line.
		EnableSingleLineStackTrace(wanted bool) (prior bool)
	}
)

const testing_lane_id testingLaneId = "testing_lane"

func NewTestingLane(ctx OptionalContext) TestingLane {
	return deriveTestingLane(ctx, nil, []Lane{})
}

func deriveTestingLane(ctx context.Context, parent *testingLane, tees []Lane) TestingLane {
	if ctx == nil {
		ctx = context.Background()
	}

	tl := testingLane{
		stackTrace: make([]atomic.Bool, logLevelMax),
		parent:     parent,
		tees:       tees,
	}
	tl.EnableStackTrace(LogLevelStack, true)
	tl.SetPanicHandler(nil)
	tl.SetOwner(&tl)

	tl.testingStack.Store(true) // enable single event stack output by default

	// make a logging instance that ultimately does logging via the lane
	tlw := testingLogWriter{tl: &tl}
	tl.tlog = log.New(&tlw, "", 0)

	if parent != nil {
		tl.onPanic = parent.onPanic
		tl.wantDescendantEvents = parent.wantDescendantEvents
		tl.journeyId = parent.journeyId
	}

	tl.Context = context.WithValue(ctx, testing_lane_id, makeLaneId())

	copyConfigToDerivation(&tl, parent)
	return &tl
}

func (tl *testingLane) SetJourneyId(id string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.journeyId = id
	// testing lane does not format a log message, so the correlation ID is ignored
}

func (tl *testingLane) SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	priorLevel = tl.level
	tl.level = newLevel
	return
}

func (tl *testingLane) VerifyEvents(eventList []*LaneEvent) bool {
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

func (tl *testingLane) FindEvents(eventList []*LaneEvent) bool {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	pos := 0
	for _, e1 := range eventList {
		found := false
		for i := pos; i < len(tl.Events); i++ {
			e2 := tl.Events[i]
			if e1.Level == e2.Level && e1.Message == e2.Message {
				pos = i + 1
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

// eventText specifies a list of events, separated by \n, and each
// line must be in the form of <level>\t<message>. Actual \n or \t
// can be specified by "\\n" or "\\t"
func (tl *testingLane) VerifyEventText(eventText string) (match bool) {
	eventList := []*LaneEvent{}

	if eventText != "" {
		lines := strings.Split(eventText, "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) != 2 {
				panic(fmt.Sprintf("eventText line must have exactly one tab separator but has %d parts: %s", len(parts), line))
			}
			text := parts[1]
			text = strings.ReplaceAll(text, "\\t", "\t")
			text = strings.ReplaceAll(text, "\\n", "\n")
			eventList = append(eventList, &LaneEvent{Level: parts[0], Message: text})
		}
	}

	return tl.VerifyEvents(eventList)
}

// eventText specifies a list of events, separated by \n, and each
// line must be in the form of <level>\t<message>.
func (tl *testingLane) FindEventText(eventText string) (found bool) {
	eventList := []*LaneEvent{}

	if eventText != "" {
		lines := strings.Split(eventText, "\n")
		for _, line := range lines {
			parts := strings.Split(line, "\t")
			if len(parts) != 2 {
				panic("eventText line must have exactly one tab separator")
			}
			eventList = append(eventList, &LaneEvent{Level: parts[0], Message: parts[1]})
		}
	}

	return tl.FindEvents(eventList)
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

func (tl *testingLane) recordLaneEvent(props loggingProperties, level LaneLogLevel, levelText string, format *string, args ...any) {
	tl.recordLaneEventRecursive(props, true, level, levelText, format, args...)
}

func (tl *testingLane) Constrain(msg string) string {
	maxLen := tl.maxLength.Load()
	if maxLen > 0 && len(msg) > int(maxLen) {
		msg = msg[:maxLen-1] + "\u2026"
	}
	return msg
}

// Worker that adds the test event to the testing lane, and then passes it up to the parent,
// where the parent decides to capture it as well, and then passes it up to the
// grandparent, and so on.
func (tl *testingLane) recordLaneEventRecursive(props loggingProperties, originator bool, level LaneLogLevel, levelText string, format *string, args ...any) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if originator || tl.wantDescendantEvents {
		if level >= tl.level {
			le := LaneEvent{
				Id:    props.laneId,
				Level: levelText,
			}

			if format == nil {
				le.Message = fmt.Sprintln(args...)          // use Sprintln because it matches log behavior wrt spaces between args
				le.Message = le.Message[:len(le.Message)-1] // remove \n
			} else {
				le.Message = fmt.Sprintf(*format, args...)
			}

			le.Message = tl.Constrain(le.Message)
			tl.Events = append(tl.Events, &le)
		}
	}

	if tl.parent != nil {
		tl.parent.recordLaneEventRecursive(props, false, level, levelText, format, args...)
	}
}

func (tl *testingLane) tee(props loggingProperties, logger teeHandler) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	for _, t := range tl.tees {
		receiver := t.(laneInternal)
		logger(props, receiver)
	}
}

func (tl *testingLane) LaneProps() loggingProperties {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	return loggingProperties{
		laneId:    tl.LaneId(),
		journeyId: tl.journeyId,
	}
}

func (tl *testingLane) Trace(args ...any) {
	tl.TraceInternal(tl.LaneProps(), args...)
}

func (tl *testingLane) Tracef(format string, args ...any) {
	tl.TracefInternal(tl.LaneProps(), format, args...)
}

func (tl *testingLane) TraceObject(message string, obj any) {
	LogObject(tl, LogLevelTrace, message, obj)
}

func (tl *testingLane) Debug(args ...any) {
	tl.DebugInternal(tl.LaneProps(), args...)
}

func (tl *testingLane) Debugf(format string, args ...any) {
	tl.DebugfInternal(tl.LaneProps(), format, args...)
}

func (tl *testingLane) DebugObject(message string, obj any) {
	LogObject(tl, LogLevelDebug, message, obj)
}

func (tl *testingLane) Info(args ...any) {
	tl.InfoInternal(tl.LaneProps(), args...)
}

func (tl *testingLane) Infof(format string, args ...any) {
	tl.InfofInternal(tl.LaneProps(), format, args...)
}

func (tl *testingLane) InfoObject(message string, obj any) {
	LogObject(tl, LogLevelInfo, message, obj)
}

func (tl *testingLane) Warn(args ...any) {
	tl.WarnInternal(tl.LaneProps(), args...)
}

func (tl *testingLane) Warnf(format string, args ...any) {
	tl.WarnfInternal(tl.LaneProps(), format, args...)
}

func (tl *testingLane) WarnObject(message string, obj any) {
	LogObject(tl, LogLevelWarn, message, obj)
}

func (tl *testingLane) Error(args ...any) {
	props := tl.LaneProps()
	tl.ErrorInternal(props, args...)
	tl.logTestingLaneStack(props, LogLevelError, 1)
}

func (tl *testingLane) Errorf(format string, args ...any) {
	props := tl.LaneProps()
	tl.ErrorfInternal(props, format, args...)
	tl.logTestingLaneStack(props, LogLevelError, 1)
}

func (tl *testingLane) ErrorObject(message string, obj any) {
	LogObject(tl, LogLevelError, message, obj)
}

func (tl *testingLane) PreFatal(args ...any) {
	tl.PreFatalInternal(tl.LaneProps(), args...)
}

func (tl *testingLane) PreFatalf(format string, args ...any) {
	tl.PreFatalfInternal(tl.LaneProps(), format, args...)
}

func (tl *testingLane) PreFatalObject(message string, obj any) {
	LogObject(tl, logLevelPreFatal, message, obj)
}

func (tl *testingLane) Fatal(args ...any) {
	tl.FatalInternal(tl.LaneProps(), args...)
	tl.onPanic()
}

func (tl *testingLane) Fatalf(format string, args ...any) {
	tl.FatalfInternal(tl.LaneProps(), format, args...)
	tl.onPanic()
}

func (tl *testingLane) FatalObject(message string, obj any) {
	LogObject(tl, LogLevelFatal, message, obj)
}

func (tl *testingLane) logTestingLaneStack(props loggingProperties, level LaneLogLevel, skippedCallers int) {
	if tl.testingStack.Load() {
		if tl.stackTrace[level].Load() {
			// When single event stack trace is enabled in the testing lane, record
			// the stack as a single message, so that the test code has a predictable
			// number of log events.
			buf := make([]byte, 16384)
			n := runtime.Stack(buf, false)
			lines := cleanStack(buf[:n], skippedCallers)

			filtered := strings.Join(lines, "\n")

			format := "%s"
			tl.recordLaneEvent(props, level, "STACK", &format, filtered)
		}
	} else {
		// When single event stack trace is not enabled in the testing lane, fall
		// back to the normal behavior
		tl.logStackIf(props, level, "", skippedCallers)
	}
}

func (tl *testingLane) logStackIf(props loggingProperties, level LaneLogLevel, message string, skippedCallers int) {

	if tl.stackTrace[level].Load() {
		// skip lines: the first line (goroutine label), plus the LogStack() and logging API
		tl.logStack(props, message, skippedCallers)
	}
}

func (tl *testingLane) logStack(props loggingProperties, message string, skippedCallers int) {
	buf := make([]byte, 16384)
	n := runtime.Stack(buf, false)
	lines := cleanStack(buf[:n], skippedCallers)

	// each has two lines (the function name on one line, followed by source info on the next line)
	format := "%s"
	if message != "" {
		tl.recordLaneEvent(props, LogLevelStack, "STACK", &format, message)
	}

	for _, line := range lines {
		tl.recordLaneEvent(props, LogLevelStack, "STACK", &format, line)
	}
}

func (tl *testingLane) LogStack(message string) {
	tl.LogStackTrim(message, 1)
}

func (tl *testingLane) LogStackTrim(message string, skippedCallers int) {
	tl.LogStackTrimInternal(tl.LaneProps(), message, skippedCallers)
}

func (tl *testingLane) SetLengthConstraint(maxLength int) int {
	old := tl.maxLength.Load()
	if maxLength > 1 {
		tl.maxLength.Store(int32(maxLength))
	} else {
		tl.maxLength.Store(0)
	}
	return int(old)
}

func (tl *testingLane) Logger() *log.Logger {
	return tl.tlog
}

func (tl *testingLane) Close() {
}

func (tl *testingLane) Derive() Lane {
	l := deriveTestingLane(context.WithValue(tl.Context, ParentLaneIdKey, tl.LaneId()), tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l
}

func (tl *testingLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(tl.Context, ParentLaneIdKey, tl.LaneId()))
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveWithCancelCause() (Lane, context.CancelCauseFunc) {
	childCtx, cancelFn := context.WithCancelCause(context.WithValue(tl.Context, ParentLaneIdKey, tl.LaneId()))
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveWithoutCancel() Lane {
	childCtx := context.WithoutCancel(context.WithValue(tl.Context, ParentLaneIdKey, tl.LaneId()))
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l
}

func (tl *testingLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadline(context.WithValue(tl.Context, ParentLaneIdKey, tl.LaneId()), deadline)
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveWithDeadlineCause(deadline time.Time, cause error) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadlineCause(context.WithValue(tl.Context, ParentLaneIdKey, tl.LaneId()), deadline, cause)
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeout(context.WithValue(tl.Context, ParentLaneIdKey, tl.LaneId()), duration)
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveWithTimeoutCause(duration time.Duration, cause error) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeoutCause(context.WithValue(tl.Context, ParentLaneIdKey, tl.LaneId()), duration, cause)
	l := deriveTestingLane(childCtx, tl, tl.tees)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	return l, cancelFn
}

func (tl *testingLane) DeriveReplaceContext(ctx OptionalContext) Lane {
	l := NewTestingLane(ctx)
	l.WantDescendantEvents(tl.wantDescendantEvents)

	tl.mu.Lock()
	defer tl.mu.Unlock()
	l.SetLogLevel(tl.level)

	for _, tee := range tl.tees {
		l.AddTee(tee)
	}

	copyConfigToDerivation(l, tl)
	return l
}

func (tl *testingLane) EnableStackTrace(level LaneLogLevel, enable bool) bool {
	return tl.stackTrace[level].Swap(enable)
}

func (tl *testingLane) EnableSingleLineStackTrace(enable bool) bool {
	return tl.testingStack.Swap(enable)
}

func (tl *testingLane) LaneId() string {
	return tl.Value(testing_lane_id).(string)
}

func (tl *testingLane) JourneyId() string {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	return tl.journeyId
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

func (tl *testingLane) Tees() []Lane {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tees := make([]Lane, len(tl.tees))
	copy(tees, tl.tees)
	return tees
}

func (tl *testingLane) SetPanicHandler(handler Panic) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if handler == nil {
		handler = func() { panic("fatal error") }
	}
	tl.onPanic = handler
}

func (tl *testingLane) Parent() Lane {
	if tl.parent != nil {
		return tl.parent
	}
	return nil // untyped nil
}

func (tlw *testingLogWriter) Write(p []byte) (n int, err error) {
	text := strings.TrimSuffix(string(p), "\n")
	tlw.tl.Info(text)
	return len(p), nil
}

func (tl *testingLane) TraceInternal(props loggingProperties, args ...any) {
	tl.recordLaneEvent(props, LogLevelTrace, "TRACE", nil, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.TraceInternal(teeProps, args...) })
}

func (tl *testingLane) TracefInternal(props loggingProperties, format string, args ...any) {
	tl.recordLaneEvent(props, LogLevelTrace, "TRACE", &format, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.TracefInternal(teeProps, format, args...) })
}

func (tl *testingLane) DebugInternal(props loggingProperties, args ...any) {
	tl.recordLaneEvent(props, LogLevelDebug, "DEBUG", nil, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.DebugInternal(teeProps, args...) })
}

func (tl *testingLane) DebugfInternal(props loggingProperties, format string, args ...any) {
	tl.recordLaneEvent(props, LogLevelDebug, "DEBUG", &format, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.DebugfInternal(teeProps, format, args...) })
}

func (tl *testingLane) InfoInternal(props loggingProperties, args ...any) {
	tl.recordLaneEvent(props, LogLevelInfo, "INFO", nil, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.InfoInternal(teeProps, args...) })
}

func (tl *testingLane) InfofInternal(props loggingProperties, format string, args ...any) {
	tl.recordLaneEvent(props, LogLevelInfo, "INFO", &format, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.InfofInternal(teeProps, format, args...) })
}

func (tl *testingLane) WarnInternal(props loggingProperties, args ...any) {
	tl.recordLaneEvent(props, LogLevelWarn, "WARN", nil, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.WarnInternal(teeProps, args...) })
}

func (tl *testingLane) WarnfInternal(props loggingProperties, format string, args ...any) {
	tl.recordLaneEvent(props, LogLevelWarn, "WARN", &format, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.WarnfInternal(teeProps, format, args...) })
}

func (tl *testingLane) ErrorInternal(props loggingProperties, args ...any) {
	tl.recordLaneEvent(props, LogLevelError, "ERROR", nil, args...)
	tl.logTestingLaneStack(props, LogLevelError, 1)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.ErrorInternal(teeProps, args...) })
}

func (tl *testingLane) ErrorfInternal(props loggingProperties, format string, args ...any) {
	tl.recordLaneEvent(props, LogLevelError, "ERROR", &format, args...)
	tl.logTestingLaneStack(props, LogLevelError, 1)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.ErrorfInternal(teeProps, format, args...) })
}

func (tl *testingLane) PreFatalInternal(props loggingProperties, args ...any) {
	tl.recordLaneEvent(props, LogLevelFatal, "FATAL", nil, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.PreFatalInternal(teeProps, args...) })
}

func (tl *testingLane) PreFatalfInternal(props loggingProperties, format string, args ...any) {
	tl.recordLaneEvent(props, LogLevelFatal, "FATAL", &format, args...)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.PreFatalfInternal(teeProps, format, args...) })
}

func (tl *testingLane) FatalInternal(props loggingProperties, args ...any) {
	tl.PreFatalInternal(props, args...)
	// panic occurs on the externally called Fatal() in a moment
}

func (tl *testingLane) FatalfInternal(props loggingProperties, format string, args ...any) {
	tl.PreFatalfInternal(props, format, args...)
	// panic occurs on the externally called Fatalf() in a moment
}

func (tl *testingLane) LogStackTrimInternal(props loggingProperties, message string, skippedCallers int) {
	tl.logStackIf(props, LogLevelStack, message, skippedCallers)
	tl.tee(props, func(teeProps loggingProperties, li laneInternal) {
		li.LogStackTrimInternal(teeProps, message, skippedCallers)
	})
}
