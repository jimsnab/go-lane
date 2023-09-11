package lane

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const null_lane_id = nullContext("null_lane_id")

type (
	nullLane struct {
		context.Context
		wlog  *log.Logger
		level int32
	}

	wrappedNullWriter struct {
		nl *nullLane
	}

	nullContext string
)

func NewNullLane(ctx context.Context) Lane {
	nl := nullLane{}

	wnw := wrappedNullWriter{nl: &nl}
	nl.wlog = log.New(&wnw, "", 0)

	nl.Context = context.WithValue(ctx, null_lane_id, uuid.New().String())
	return &nl
}

func (nl *nullLane) SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel) {
	level := int32(newLevel)
	priorLevel = LaneLogLevel(atomic.SwapInt32(&nl.level, level))
	return
}

func (nl *nullLane) Trace(args ...any)                 {}
func (nl *nullLane) Tracef(format string, args ...any) {}
func (nl *nullLane) Debug(args ...any)                 {}
func (nl *nullLane) Debugf(format string, args ...any) {}
func (nl *nullLane) Info(args ...any)                  {}
func (nl *nullLane) Infof(format string, args ...any)  {}
func (nl *nullLane) Warn(args ...any)                  {}
func (nl *nullLane) Warnf(format string, args ...any)  {}
func (nl *nullLane) Error(args ...any)                 {}
func (nl *nullLane) Errorf(format string, args ...any) {}
func (nl *nullLane) Fatal(args ...any)                 { panic("fatal error") }
func (nl *nullLane) Fatalf(format string, args ...any) { panic("fatal error") }

func (nl *nullLane) Logger() *log.Logger {
	return nl.wlog
}

func (nl *nullLane) Close() {
}

func (nl *nullLane) Derive() Lane {
	l := NewTestingLane(context.WithValue(nl.Context, parent_lane_id, nl.LaneId()))
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l
}

func (nl *nullLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(nl.Context, parent_lane_id, nl.LaneId()))
	l := NewNullLane(childCtx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadline(context.WithValue(nl.Context, parent_lane_id, nl.LaneId()), deadline)
	l := NewNullLane(childCtx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeout(context.WithValue(nl.Context, parent_lane_id, nl.LaneId()), duration)
	l := NewNullLane(childCtx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveReplaceContext(ctx context.Context) Lane {
	l := NewNullLane(ctx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l
}

func (nl *nullLane) LaneId() string {
	return nl.Value(null_lane_id).(string)
}

func (wnw *wrappedNullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
