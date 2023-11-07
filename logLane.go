package lane

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type (
	logLane struct {
		context.Context
		wlog       *log.Logger
		writer     *log.Logger
		level      int32
		cr         string
		stackTrace []atomic.Bool
	}

	wrappedLogWriter struct {
		ll *logLane
	}

	laneId string
)

const log_lane_id = laneId("log_lane_id")
const parent_lane_id = laneId("parent_lane_id")

func isLogCrLf() bool {
	var buf bytes.Buffer
	testLog := log.New(&buf, "", 0)
	testLog.Println()
	return buf.Bytes()[0] == '\r'
}

func NewLogLane(ctx context.Context) Lane {
	return newLogLane(ctx)
}

func newLogLane(ctx context.Context) *logLane {
	ll := logLane{
		stackTrace: make([]atomic.Bool, int(LogLevelFatal+1)),
	}

	// make a logging instance that ultimately does logging via the lane
	wlw := wrappedLogWriter{ll: &ll}
	ll.writer = log.Default()
	ll.wlog = log.New(&wlw, "", 0)

	id := uuid.New().String()
	id = id[len(id)-10:]
	ll.Context = context.WithValue(ctx, log_lane_id, id)
	return &ll
}

// For cases where \r\n line endings are required (ex: vscode terminal)
func NewLogLaneWithCR(ctx context.Context) Lane {
	ll := newLogLane(ctx)

	if !isLogCrLf() {
		ll.cr = "\r"
	}

	return ll
}

func sprint(args ...any) string {
	// fmt.Sprint doesn't insert spaces the same as fmt.Sprintln, but we don't
	// want the line ending
	text := fmt.Sprintln(args...)
	text = strings.TrimSuffix(text, "\n")
	return text
}

func (ll *logLane) SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel) {
	level := int32(newLevel)
	priorLevel = LaneLogLevel(atomic.SwapInt32(&ll.level, level))
	return
}

func (ll *logLane) getLaneId(level string) string {
	return fmt.Sprintf("%s {%s}", level, ll.LaneId())
}

func (ll *logLane) shouldLog(level LaneLogLevel) bool {
	return atomic.LoadInt32(&ll.level) <= int32(level)
}

func (ll *logLane) Trace(args ...any) {
	if ll.shouldLog(LogLevelTrace) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("TRACE"), sprint(args...), ll.cr)
		ll.logStack(LogLevelTrace)
	}
}

func (ll *logLane) Tracef(format string, args ...any) {
	if ll.shouldLog(LogLevelTrace) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("TRACE"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelTrace)
	}
}

func (ll *logLane) Debug(args ...any) {
	if ll.shouldLog(LogLevelDebug) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("DEBUG"), sprint(args...), ll.cr)
		ll.logStack(LogLevelDebug)
	}
}

func (ll *logLane) Debugf(format string, args ...any) {
	if ll.shouldLog(LogLevelDebug) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("DEBUG"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelDebug)
	}
}

func (ll *logLane) Info(args ...any) {
	if ll.shouldLog(LogLevelInfo) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("INFO"), sprint(args...), ll.cr)
		ll.logStack(LogLevelInfo)
	}
}

func (ll *logLane) Infof(format string, args ...any) {
	if ll.shouldLog(LogLevelInfo) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("INFO"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelInfo)
	}
}

func (ll *logLane) Warn(args ...any) {
	if ll.shouldLog(LogLevelWarn) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("WARN"), sprint(args...), ll.cr)
		ll.logStack(LogLevelWarn)
	}
}

func (ll *logLane) Warnf(format string, args ...any) {
	if ll.shouldLog(LogLevelWarn) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("WARN"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelWarn)
	}
}

func (ll *logLane) Error(args ...any) {
	if ll.shouldLog(LogLevelError) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("ERROR"), sprint(args...), ll.cr)
		ll.logStack(LogLevelError)
	}
}

func (ll *logLane) Errorf(format string, args ...any) {
	if ll.shouldLog(LogLevelError) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("ERROR"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelError)
	}
}

func (ll *logLane) Fatal(args ...any) {
	if ll.shouldLog(LogLevelFatal) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("FATAL"), sprint(args...), ll.cr)
		ll.logStack(LogLevelFatal)
	}
	panic("fatal error")
}

func (ll *logLane) Fatalf(format string, args ...any) {
	if ll.shouldLog(LogLevelFatal) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("FATAL"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelFatal)
	}
	panic("fatal error")
}

func (ll *logLane) logStack(level LaneLogLevel) {
	if ll.stackTrace[level].Load() {
		buf := make([]byte, 16384)
		n := runtime.Stack(buf, false)
		lines := strings.Split(strings.TrimSpace(string(buf[0:n])), "\n")

		// skip five lines: the first line (goroutine label), plus the logStack() and logging API,
		// each has two lines (the function name on one line, followed by source info on the next line)
		for n := 5; n < len(lines); n++ {
			ll.writer.Printf("%s %s%s", ll.getLaneId("STACK"), lines[n], ll.cr)
		}
	}
}

func (ll *logLane) Logger() *log.Logger {
	return ll.wlog
}

func (ll *logLane) Close() {
}

func (ll *logLane) Derive() Lane {
	l := NewLogLane(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()))
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l
}

func (ll *logLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()))
	l := NewLogLane(childCtx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l, cancelFn
}

func (ll *logLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadline(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()), deadline)
	l := NewLogLane(childCtx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l, cancelFn
}

func (ll *logLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeout(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()), duration)
	l := NewLogLane(childCtx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l, cancelFn
}

func (ll *logLane) DeriveReplaceContext(ctx context.Context) Lane {
	l := NewLogLane(ctx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l
}

func (ll *logLane) LaneId() string {
	return ll.Value(log_lane_id).(string)
}

func (ll *logLane) EnableStackTrace(level LaneLogLevel, enable bool) bool {
	return ll.stackTrace[level].Swap(enable)
}

func (wlw *wrappedLogWriter) Write(p []byte) (n int, err error) {
	text := string(p)
	wlw.ll.Info(text)
	return len(p), nil
}
