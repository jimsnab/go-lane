package lane

import (
	"bytes"
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
	logLane struct {
		// Be sure to update clone() if modifying this struct
		context.Context
		wlog       *log.Logger
		writer     *log.Logger
		level      int32
		cr         string
		stackTrace []atomic.Bool
		mu         sync.Mutex
		tees       []Lane
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
	return deriveLogLane(ctx, []Lane{}, "")
}

func deriveLogLane(ctx context.Context, tees []Lane, cr string) *logLane {
	ll := logLane{
		stackTrace: make([]atomic.Bool, int(LogLevelFatal+1)),
		tees:       tees,
		cr:         cr,
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

// clone - any lane that is mostly a log lane needs this, such as diskLane
func (ll *logLane) clone(ll2 *logLane) {
	// this method is needed to avoid copy of mu
	ll2.Context = ll.Context
	ll2.wlog = ll.wlog
	ll2.writer = ll.writer
	ll2.level = ll.level
	ll2.cr = ll.cr
	ll2.stackTrace = ll.stackTrace
	ll2.tees = ll.tees
}

// For cases where \r\n line endings are required (ex: vscode terminal)
func NewLogLaneWithCR(ctx context.Context) Lane {
	cr := ""
	if !isLogCrLf() {
		cr = "\r"
	}
	return deriveLogLane(ctx, []Lane{}, cr)
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

func (ll *logLane) tee(logger func(l Lane)) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	for _, t := range ll.tees {
		logger(t)
	}
}

func (ll *logLane) Trace(args ...any) {
	if ll.shouldLog(LogLevelTrace) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("TRACE"), sprint(args...), ll.cr)
		ll.logStack(LogLevelTrace)
	}
	ll.tee(func(l Lane) { l.Trace(args...) })
}

func (ll *logLane) Tracef(format string, args ...any) {
	if ll.shouldLog(LogLevelTrace) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("TRACE"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelTrace)
	}
	ll.tee(func(l Lane) { l.Tracef(format, args...) })
}

func (ll *logLane) Debug(args ...any) {
	if ll.shouldLog(LogLevelDebug) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("DEBUG"), sprint(args...), ll.cr)
		ll.logStack(LogLevelDebug)
	}
	ll.tee(func(l Lane) { l.Debug(args...) })
}

func (ll *logLane) Debugf(format string, args ...any) {
	if ll.shouldLog(LogLevelDebug) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("DEBUG"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelDebug)
	}
	ll.tee(func(l Lane) { l.Debugf(format, args...) })
}

func (ll *logLane) Info(args ...any) {
	if ll.shouldLog(LogLevelInfo) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("INFO"), sprint(args...), ll.cr)
		ll.logStack(LogLevelInfo)
	}
	ll.tee(func(l Lane) { l.Info(args...) })
}

func (ll *logLane) Infof(format string, args ...any) {
	if ll.shouldLog(LogLevelInfo) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("INFO"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelInfo)
	}
	ll.tee(func(l Lane) { l.Infof(format, args...) })
}

func (ll *logLane) Warn(args ...any) {
	if ll.shouldLog(LogLevelWarn) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("WARN"), sprint(args...), ll.cr)
		ll.logStack(LogLevelWarn)
	}
	ll.tee(func(l Lane) { l.Warn(args...) })
}

func (ll *logLane) Warnf(format string, args ...any) {
	if ll.shouldLog(LogLevelWarn) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("WARN"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelWarn)
	}
	ll.tee(func(l Lane) { l.Warnf(format, args...) })
}

func (ll *logLane) Error(args ...any) {
	if ll.shouldLog(LogLevelError) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("ERROR"), sprint(args...), ll.cr)
		ll.logStack(LogLevelError)
	}
	ll.tee(func(l Lane) { l.Error(args...) })
}

func (ll *logLane) Errorf(format string, args ...any) {
	if ll.shouldLog(LogLevelError) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("ERROR"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelError)
	}
	ll.tee(func(l Lane) { l.Errorf(format, args...) })
}

func (ll *logLane) PreFatal(args ...any) {
	if ll.shouldLog(LogLevelFatal) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("FATAL"), sprint(args...), ll.cr)
		ll.logStack(LogLevelFatal)
	}
	ll.tee(func(l Lane) { l.PreFatal(args...) })
}

func (ll *logLane) PreFatalf(format string, args ...any) {
	if ll.shouldLog(LogLevelFatal) {
		ll.writer.Printf("%s %s%s", ll.getLaneId("FATAL"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelFatal)
	}
	ll.tee(func(l Lane) { l.PreFatalf(format, args...) })
}

func (ll *logLane) Fatal(args ...any) {
	ll.PreFatal(args...)
	panic("fatal error")
}

func (ll *logLane) Fatalf(format string, args ...any) {
	ll.PreFatalf(format, args...)
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
	l := deriveLogLane(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()), ll.tees, ll.cr)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l
}

func (ll *logLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()))
	l := deriveLogLane(childCtx, ll.tees, ll.cr)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l, cancelFn
}

func (ll *logLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadline(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()), deadline)
	l := deriveLogLane(childCtx, ll.tees, ll.cr)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l, cancelFn
}

func (ll *logLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeout(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()), duration)
	l := deriveLogLane(childCtx, ll.tees, ll.cr)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l, cancelFn
}

func (ll *logLane) DeriveReplaceContext(ctx context.Context) Lane {
	l := deriveLogLane(ctx, ll.tees, ll.cr)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))
	return l
}

func (ll *logLane) LaneId() string {
	return ll.Value(log_lane_id).(string)
}

func (ll *logLane) EnableStackTrace(level LaneLogLevel, enable bool) bool {
	return ll.stackTrace[level].Swap(enable)
}

func (ll *logLane) AddTee(l Lane) {
	ll.mu.Lock()
	ll.tees = append(ll.tees, l)
	ll.mu.Unlock()
}

func (ll *logLane) RemoveTee(l Lane) {
	ll.mu.Lock()
	for i, t := range ll.tees {
		if t.LaneId() == l.LaneId() {
			ll.tees = append(ll.tees[:i], ll.tees[i+1:]...)
			break
		}
	}
	ll.mu.Unlock()
}

func (wlw *wrappedLogWriter) Write(p []byte) (n int, err error) {
	text := string(p)
	wlw.ll.Info(text)
	return len(p), nil
}
