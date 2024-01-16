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
		// !!! Be sure to update clone() if modifying this struct
		context.Context
		wlog       *log.Logger // wrapper log to capture caller's logging intent without sending to output
		writer     *log.Logger // the log instance used for output
		level      int32
		cr         string
		stackTrace []atomic.Bool
		mu         sync.Mutex
		tees       []Lane
		journeyId  string
		// !!! Be sure to update clone() if modifying this struct
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
	return deriveLogLane(nil, ctx, []Lane{}, "")
}

func deriveLogLane(parent *logLane, ctx context.Context, tees []Lane, cr string) *logLane {
	ll := logLane{
		stackTrace: make([]atomic.Bool, int(LogLevelFatal+1)),
		tees:       tees,
		cr:         cr,
	}

	// make a logging instance that ultimately does logging via the lane
	wlw := wrappedLogWriter{ll: &ll}
	ll.writer = log.Default()
	ll.wlog = log.New(&wlw, "", 0)

	if parent != nil {
		ll.journeyId = parent.journeyId
		ll.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&parent.level)))
		ll.wlog.SetFlags(parent.wlog.Flags())
		ll.wlog.SetPrefix(parent.wlog.Prefix())
	}

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
	ll2.journeyId = ll.journeyId
}

// For cases where \r\n line endings are required (ex: vscode terminal)
func NewLogLaneWithCR(ctx context.Context) Lane {
	cr := ""
	if !isLogCrLf() {
		cr = "\r"
	}
	return deriveLogLane(nil, ctx, []Lane{}, cr)
}

// Adds an ID to the log message(s)
func (ll *logLane) SetJourneyId(id string) {
	if len(id) > 10 {
		ll.journeyId = id[:10]
	} else {
		ll.journeyId = id
	}
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

func (ll *logLane) getMesagePrefix(level string) string {
	if ll.journeyId != "" {
		return fmt.Sprintf("%s {%s:%s}", level, ll.journeyId, ll.LaneId())
	} else {
		return fmt.Sprintf("%s {%s}", level, ll.LaneId())
	}
}

func (ll *logLane) shouldLog(level LaneLogLevel) bool {
	if atomic.LoadInt32(&ll.level) <= int32(level) {
		// the log wrapper is exposed to the client, so ensure changes
		// made to prefix and flags are copied into the instance
		// generating the output
		ll.writer.SetPrefix(ll.wlog.Prefix())
		ll.writer.SetFlags(ll.wlog.Flags())
		return true
	}

	return false
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
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("TRACE"), sprint(args...), ll.cr)
		ll.logStack(LogLevelTrace)
	}
	ll.tee(func(l Lane) { l.Trace(args...) })
}

func (ll *logLane) Tracef(format string, args ...any) {
	if ll.shouldLog(LogLevelTrace) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("TRACE"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelTrace)
	}
	ll.tee(func(l Lane) { l.Tracef(format, args...) })
}

func (ll *logLane) Debug(args ...any) {
	if ll.shouldLog(LogLevelDebug) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("DEBUG"), sprint(args...), ll.cr)
		ll.logStack(LogLevelDebug)
	}
	ll.tee(func(l Lane) { l.Debug(args...) })
}

func (ll *logLane) Debugf(format string, args ...any) {
	if ll.shouldLog(LogLevelDebug) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("DEBUG"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelDebug)
	}
	ll.tee(func(l Lane) { l.Debugf(format, args...) })
}

func (ll *logLane) Info(args ...any) {
	if ll.shouldLog(LogLevelInfo) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("INFO"), sprint(args...), ll.cr)
		ll.logStack(LogLevelInfo)
	}
	ll.tee(func(l Lane) { l.Info(args...) })
}

func (ll *logLane) Infof(format string, args ...any) {
	if ll.shouldLog(LogLevelInfo) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("INFO"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelInfo)
	}
	ll.tee(func(l Lane) { l.Infof(format, args...) })
}

func (ll *logLane) Warn(args ...any) {
	if ll.shouldLog(LogLevelWarn) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("WARN"), sprint(args...), ll.cr)
		ll.logStack(LogLevelWarn)
	}
	ll.tee(func(l Lane) { l.Warn(args...) })
}

func (ll *logLane) Warnf(format string, args ...any) {
	if ll.shouldLog(LogLevelWarn) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("WARN"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelWarn)
	}
	ll.tee(func(l Lane) { l.Warnf(format, args...) })
}

func (ll *logLane) Error(args ...any) {
	if ll.shouldLog(LogLevelError) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("ERROR"), sprint(args...), ll.cr)
		ll.logStack(LogLevelError)
	}
	ll.tee(func(l Lane) { l.Error(args...) })
}

func (ll *logLane) Errorf(format string, args ...any) {
	if ll.shouldLog(LogLevelError) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("ERROR"), fmt.Sprintf(format, args...), ll.cr)
		ll.logStack(LogLevelError)
	}
	ll.tee(func(l Lane) { l.Errorf(format, args...) })
}

func (ll *logLane) PreFatal(args ...any) {
	if ll.shouldLog(LogLevelFatal) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("FATAL"), sprint(args...), ll.cr)
		ll.logStack(LogLevelFatal)
	}
	ll.tee(func(l Lane) { l.PreFatal(args...) })
}

func (ll *logLane) PreFatalf(format string, args ...any) {
	if ll.shouldLog(LogLevelFatal) {
		ll.writer.Printf("%s %s%s", ll.getMesagePrefix("FATAL"), fmt.Sprintf(format, args...), ll.cr)
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
			ll.writer.Printf("%s %s%s", ll.getMesagePrefix("STACK"), lines[n], ll.cr)
		}
	}
}

func (ll *logLane) Logger() *log.Logger {
	return ll.wlog
}

func (ll *logLane) Close() {
}

func (ll *logLane) Derive() Lane {
	return deriveLogLane(ll, context.WithValue(ll.Context, parent_lane_id, ll.LaneId()), ll.tees, ll.cr)
}

func (ll *logLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()))
	l := deriveLogLane(ll, childCtx, ll.tees, ll.cr)
	return l, cancelFn
}

func (ll *logLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadline(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()), deadline)
	l := deriveLogLane(ll, childCtx, ll.tees, ll.cr)
	return l, cancelFn
}

func (ll *logLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeout(context.WithValue(ll.Context, parent_lane_id, ll.LaneId()), duration)
	l := deriveLogLane(ll, childCtx, ll.tees, ll.cr)
	return l, cancelFn
}

func (ll *logLane) DeriveReplaceContext(ctx context.Context) Lane {
	return deriveLogLane(ll, ctx, ll.tees, ll.cr)
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
