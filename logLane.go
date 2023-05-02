package lane

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type (
	logLane struct {
		context.Context
		wlog  *log.Logger
		level int32
	}

	wrappedLogWriter struct {
		ll *logLane
	}

	laneId string
)

const log_lane_id = laneId("log_lane_id")
const parent_lane_id = laneId("parent_lane_id")

func NewLogLane(ctx context.Context) Lane {
	ll := logLane{}

	// make a logging instance that ultimately does logging via the lane
	wlw := wrappedLogWriter{ll: &ll}
	ll.wlog = log.New(&wlw, "", 0)

	id := uuid.New().String()
	id = id[len(id)-10:]
	ll.Context = context.WithValue(ctx, log_lane_id, id)
	return &ll
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
		log.Printf("%s %s", ll.getLaneId("TRACE"), fmt.Sprintln(args...))
	}
}

func (ll *logLane) Tracef(format string, args ...any) {
	if ll.shouldLog(LogLevelTrace) {
		log.Println(ll.getLaneId("TRACE"), fmt.Sprintf(format, args...))
	}
}

func (ll *logLane) Debug(args ...any) {
	if ll.shouldLog(LogLevelDebug) {
		log.Printf("%s %s", ll.getLaneId("DEBUG"), fmt.Sprintln(args...))
	}
}

func (ll *logLane) Debugf(format string, args ...any) {
	if ll.shouldLog(LogLevelDebug) {
		log.Println(ll.getLaneId("DEBUG"), fmt.Sprintf(format, args...))
	}
}

func (ll *logLane) Info(args ...any) {
	if ll.shouldLog(LogLevelInfo) {
		log.Printf("%s %s", ll.getLaneId("INFO"), fmt.Sprintln(args...))
	}
}

func (ll *logLane) Infof(format string, args ...any) {
	if ll.shouldLog(LogLevelInfo) {
		log.Println(ll.getLaneId("INFO"), fmt.Sprintf(format, args...))
	}
}

func (ll *logLane) Warn(args ...any) {
	if ll.shouldLog(LogLevelWarn) {
		log.Printf("%s %s", ll.getLaneId("WARN"), fmt.Sprintln(args...))
	}
}

func (ll *logLane) Warnf(format string, args ...any) {
	if ll.shouldLog(LogLevelWarn) {
		log.Println(ll.getLaneId("WARN"), fmt.Sprintf(format, args...))
	}
}

func (ll *logLane) Error(args ...any) {
	if ll.shouldLog(LogLevelError) {
		log.Printf("%s %s", ll.getLaneId("ERROR"), fmt.Sprintln(args...))
	}
}

func (ll *logLane) Errorf(format string, args ...any) {
	if ll.shouldLog(LogLevelError) {
		log.Println(ll.getLaneId("ERROR"), fmt.Sprintf(format, args...))
	}
}

func (ll *logLane) Fatal(args ...any) {
	if ll.shouldLog(LogLevelFatal) {
		log.Printf("%s %s", ll.getLaneId("FATAL"), fmt.Sprintln(args...))
	}
	panic("fatal error")
}

func (ll *logLane) Fatalf(format string, args ...any) {
	if ll.shouldLog(LogLevelFatal) {
		log.Println(ll.getLaneId("FATAL"), fmt.Sprintf(format, args...))
	}
	panic("fatal error")
}

func (ll *logLane) Logger() *log.Logger {
	return ll.wlog
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

func (ll *logLane) LaneId() string {
	return ll.Value(log_lane_id).(string)
}

func (wlw *wrappedLogWriter) Write(p []byte) (n int, err error) {
	text := string(p)
	wlw.ll.Info(text)
	return len(p), nil
}
