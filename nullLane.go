package lane

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const null_lane_id = nullContext("null_lane_id")

type (
	nullLane struct {
		context.Context
		wlog       *log.Logger
		level      int32
		stackTrace []atomic.Bool
		mu         sync.Mutex
		tees       []Lane
		onPanic    Panic
		journeyId  string
	}

	wrappedNullWriter struct {
		nl *nullLane
	}

	nullContext string
)

func NewNullLane(ctx context.Context) Lane {
	return deriveNullLane(ctx, []Lane{}, nil)
}

func deriveNullLane(ctx context.Context, tees []Lane, onPanic Panic) Lane {
	nl := nullLane{
		stackTrace: make([]atomic.Bool, int(LogLevelFatal+1)),
		tees:       tees,
	}
	nl.SetPanicHandler(onPanic)

	wnw := wrappedNullWriter{nl: &nl}
	nl.wlog = log.New(&wnw, "", 0)

	nl.Context = context.WithValue(ctx, null_lane_id, uuid.New().String())
	return &nl
}

func (nl *nullLane) SetJourneyId(id string) {
	nl.mu.Lock()
	defer nl.mu.Unlock()
	nl.journeyId = id
	// null lane does not format a log message, so the correlation ID is ignored
}

func (nl *nullLane) SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel) {
	level := int32(newLevel)
	priorLevel = LaneLogLevel(atomic.SwapInt32(&nl.level, level))
	return
}

func (nl *nullLane) tee(logger func(l Lane)) {
	nl.mu.Lock()
	defer nl.mu.Unlock()

	for _, t := range nl.tees {
		logger(t)
	}
}

func (nl *nullLane) Trace(args ...any) { nl.tee(func(l Lane) { l.Trace(args...) }) }
func (nl *nullLane) Tracef(format string, args ...any) {
	nl.tee(func(l Lane) { l.Tracef(format, args...) })
}
func (nl *nullLane) Debug(args ...any) { nl.tee(func(l Lane) { l.Debug(args...) }) }
func (nl *nullLane) Debugf(format string, args ...any) {
	nl.tee(func(l Lane) { l.Debugf(format, args...) })
}
func (nl *nullLane) Info(args ...any) { nl.tee(func(l Lane) { l.Info(args...) }) }
func (nl *nullLane) Infof(format string, args ...any) {
	nl.tee(func(l Lane) { l.Infof(format, args...) })
}
func (nl *nullLane) Warn(args ...any) { nl.tee(func(l Lane) { l.Warn(args...) }) }
func (nl *nullLane) Warnf(format string, args ...any) {
	nl.tee(func(l Lane) { l.Warnf(format, args...) })
}
func (nl *nullLane) Error(args ...any) { nl.tee(func(l Lane) { l.Error(args...) }) }
func (nl *nullLane) Errorf(format string, args ...any) {
	nl.tee(func(l Lane) { l.Errorf(format, args...) })
}
func (nl *nullLane) PreFatal(args ...any) { nl.tee(func(l Lane) { l.PreFatal(args...) }) }
func (nl *nullLane) PreFatalf(format string, args ...any) {
	nl.tee(func(l Lane) { l.PreFatalf(format, args...) })
}
func (nl *nullLane) Fatal(args ...any) { nl.PreFatal(args...); nl.onPanic() }
func (nl *nullLane) Fatalf(format string, args ...any) {
	nl.PreFatalf(format, args...)
	nl.onPanic()
}

func (nl *nullLane) Logger() *log.Logger {
	return nl.wlog
}

func (nl *nullLane) Close() {
}

func (nl *nullLane) Derive() Lane {
	l := deriveNullLane(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()), nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l
}

func (nl *nullLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()))
	l := deriveNullLane(childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadline(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()), deadline)
	l := deriveNullLane(childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeout(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()), duration)
	l := deriveNullLane(childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveReplaceContext(ctx context.Context) Lane {
	l := NewNullLane(ctx)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	for _, tee := range nl.tees {
		l.AddTee(tee)
	}
	return l
}

func (nl *nullLane) EnableStackTrace(level LaneLogLevel, enable bool) bool {
	// the last value should work as if the setting does something
	return nl.stackTrace[level].Swap(enable)
}

func (nl *nullLane) LaneId() string {
	return nl.Value(null_lane_id).(string)
}

func (nl *nullLane) JourneyId() string {
	nl.mu.Lock()
	defer nl.mu.Unlock()
	return nl.journeyId
}

func (nl *nullLane) AddTee(l Lane) {
	nl.mu.Lock()
	nl.tees = append(nl.tees, l)
	nl.mu.Unlock()
}

func (nl *nullLane) RemoveTee(l Lane) {
	nl.mu.Lock()
	for i, t := range nl.tees {
		if t.LaneId() == l.LaneId() {
			nl.tees = append(nl.tees[:i], nl.tees[i+1:]...)
			break
		}
	}
	nl.mu.Unlock()
}

func (nl *nullLane) SetPanicHandler(handler Panic) {
	nl.mu.Lock()
	defer nl.mu.Unlock()

	if handler == nil {
		handler = func() { panic("fatal error") }
	}
	nl.onPanic = handler
}

func (wnw *wrappedNullWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
