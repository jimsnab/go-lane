package lane

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const null_lane_id = nullContext("null_lane_id")

type (
	nullLane struct {
		context.Context
		MetadataStore
		wlog       *log.Logger
		level      int32
		stackTrace []atomic.Bool
		mu         sync.Mutex
		tees       []Lane
		onPanic    Panic
		journeyId  string
		parent     Lane
		maxLength  atomic.Int32
	}

	wrappedNullWriter struct {
		nl *nullLane
	}

	nullContext string
)

// NewNullLane creates a new lane that discards all log output.
func NewNullLane(ctx OptionalContext) Lane {
	return deriveNullLane(nil, ctx, []Lane{}, nil)
}

func deriveNullLane(parent Lane, ctx context.Context, tees []Lane, onPanic Panic) Lane {
	if ctx == nil {
		ctx = context.Background()
	}

	nl := nullLane{
		stackTrace: make([]atomic.Bool, logLevelMax),
		tees:       tees,
		parent:     parent,
	}
	nl.SetPanicHandler(onPanic)
	nl.SetOwner(&nl)

	wnw := wrappedNullWriter{nl: &nl}
	nl.wlog = log.New(&wnw, "", 0)

	nl.Context = context.WithValue(ctx, null_lane_id, makeLaneId())

	copyConfigToDerivation(&nl, parent)
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

func (nl *nullLane) IsLevelEnabled(level LaneLogLevel) bool {
	return false
}

func (nl *nullLane) tee(props loggingProperties, logger teeHandler) {
	nl.mu.Lock()
	defer nl.mu.Unlock()

	for _, t := range nl.tees {
		receiver := t.(laneInternal)
		logger(props, receiver)
	}
}

func (nl *nullLane) LaneProps() loggingProperties {
	nl.mu.Lock()
	defer nl.mu.Unlock()
	return loggingProperties{
		laneId:    nl.LaneId(),
		journeyId: nl.journeyId,
	}
}

func (nl *nullLane) Trace(args ...any) { nl.TraceInternal(nl.LaneProps(), args...) }
func (nl *nullLane) Tracef(format string, args ...any) {
	nl.TracefInternal(nl.LaneProps(), format, args...)
}
func (nl *nullLane) TraceObject(message string, obj any) {
	LogObject(nl, LogLevelTrace, message, obj)
}
func (nl *nullLane) Debug(args ...any) { nl.DebugInternal(nl.LaneProps(), args...) }
func (nl *nullLane) Debugf(format string, args ...any) {
	nl.DebugfInternal(nl.LaneProps(), format, args...)
}
func (nl *nullLane) DebugObject(message string, obj any) {
	LogObject(nl, LogLevelDebug, message, obj)
}
func (nl *nullLane) Info(args ...any) { nl.InfoInternal(nl.LaneProps(), args...) }
func (nl *nullLane) Infof(format string, args ...any) {
	nl.InfofInternal(nl.LaneProps(), format, args...)
}
func (nl *nullLane) InfoObject(message string, obj any) {
	LogObject(nl, LogLevelInfo, message, obj)
}
func (nl *nullLane) Warn(args ...any) { nl.WarnInternal(nl.LaneProps(), args...) }
func (nl *nullLane) Warnf(format string, args ...any) {
	nl.WarnfInternal(nl.LaneProps(), format, args...)
}
func (nl *nullLane) WarnObject(message string, obj any) {
	LogObject(nl, LogLevelWarn, message, obj)
}
func (nl *nullLane) Error(args ...any) { nl.ErrorInternal(nl.LaneProps(), args...) }
func (nl *nullLane) Errorf(format string, args ...any) {
	nl.ErrorfInternal(nl.LaneProps(), format, args...)
}
func (nl *nullLane) ErrorObject(message string, obj any) {
	LogObject(nl, LogLevelError, message, obj)
}
func (nl *nullLane) PreFatal(args ...any) { nl.PreFatalInternal(nl.LaneProps(), args...) }
func (nl *nullLane) PreFatalf(format string, args ...any) {
	nl.PreFatalfInternal(nl.LaneProps(), format, args...)
}
func (nl *nullLane) PreFatalObject(message string, obj any) {
	LogObject(nl, logLevelPreFatal, message, obj)
}
func (nl *nullLane) Fatal(args ...any) { nl.FatalInternal(nl.LaneProps(), args...); nl.onPanic() }
func (nl *nullLane) Fatalf(format string, args ...any) {
	nl.FatalfInternal(nl.LaneProps(), format, args...)
	nl.onPanic()
}
func (nl *nullLane) FatalObject(message string, obj any) {
	LogObject(nl, LogLevelFatal, message, obj)
}

func (nl *nullLane) LogStack(message string) {
	nl.LogStackTrim(message, 0)
}

func (nl *nullLane) LogStackTrim(message string, skippedCallers int) {
	nl.LogStackTrimInternal(nl.LaneProps(), message, skippedCallers)
}

func (nl *nullLane) SetLengthConstraint(maxLength int) int {
	old := nl.maxLength.Load()
	if maxLength > 1 {
		nl.maxLength.Store(int32(maxLength))
	} else {
		nl.maxLength.Store(0)
	}
	return int(old)
}

func (nl *nullLane) Constrain(text string) string {
	maxLen := nl.maxLength.Load()
	if maxLen > 0 && len(text) > int(maxLen) {
		text = text[:maxLen-1] + "\u2026"
	}
	return text
}

func (nl *nullLane) Logger() *log.Logger {
	return nl.wlog
}

func (nl *nullLane) Close() {
}

func (nl *nullLane) Derive() Lane {
	l := deriveNullLane(nl, context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()), nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	l.SetJourneyId(nl.journeyId)
	return l
}

func (nl *nullLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()))
	l := deriveNullLane(nl, childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithCancelCause() (Lane, context.CancelCauseFunc) {
	childCtx, cancelFn := context.WithCancelCause(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()))
	l := deriveNullLane(nl, childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithoutCancel() Lane {
	childCtx := context.WithoutCancel(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()))
	l := deriveNullLane(nl, childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l
}

func (nl *nullLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadline(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()), deadline)
	l := deriveNullLane(nl, childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithDeadlineCause(deadline time.Time, cause error) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithDeadlineCause(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()), deadline, cause)
	l := deriveNullLane(nl, childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeout(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()), duration)
	l := deriveNullLane(nl, childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveWithTimeoutCause(duration time.Duration, cause error) (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithTimeoutCause(context.WithValue(nl.Context, ParentLaneIdKey, nl.LaneId()), duration, cause)
	l := deriveNullLane(nl, childCtx, nl.tees, nl.onPanic)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
	return l, cancelFn
}

func (nl *nullLane) DeriveReplaceContext(ctx OptionalContext) Lane {
	l := deriveNullLane(nl, ctx, append([]Lane{}, nl.tees...), nil)
	l.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&nl.level)))
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

func (nl *nullLane) Tees() []Lane {
	nl.mu.Lock()
	defer nl.mu.Unlock()
	tees := make([]Lane, len(nl.tees))
	copy(tees, nl.tees)
	return tees
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

func (nl *nullLane) Parent() Lane {
	if nl.parent != nil {
		return nl.parent
	}
	return nil // untyped nil
}

func (nl *nullLane) TraceInternal(props loggingProperties, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.TraceInternal(teeProps, args...) })
}
func (nl *nullLane) TracefInternal(props loggingProperties, format string, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.TracefInternal(teeProps, format, args...) })
}
func (nl *nullLane) DebugInternal(props loggingProperties, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.DebugInternal(teeProps, args...) })
}
func (nl *nullLane) DebugfInternal(props loggingProperties, format string, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.DebugfInternal(teeProps, format, args...) })
}
func (nl *nullLane) InfoInternal(props loggingProperties, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.InfoInternal(teeProps, args...) })
}
func (nl *nullLane) InfofInternal(props loggingProperties, format string, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.InfofInternal(teeProps, format, args...) })
}
func (nl *nullLane) WarnInternal(props loggingProperties, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.WarnInternal(teeProps, args...) })
}
func (nl *nullLane) WarnfInternal(props loggingProperties, format string, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.WarnfInternal(teeProps, format, args...) })
}
func (nl *nullLane) ErrorInternal(props loggingProperties, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.ErrorInternal(teeProps, args...) })
}
func (nl *nullLane) ErrorfInternal(props loggingProperties, format string, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.ErrorfInternal(teeProps, format, args...) })
}
func (nl *nullLane) PreFatalInternal(props loggingProperties, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.PreFatalInternal(teeProps, args...) })
}
func (nl *nullLane) PreFatalfInternal(props loggingProperties, format string, args ...any) {
	nl.tee(props, func(teeProps loggingProperties, li laneInternal) { li.PreFatalfInternal(teeProps, format, args...) })
}
func (nl *nullLane) FatalInternal(props loggingProperties, args ...any) {
	nl.PreFatalInternal(props, args...)
	// panic will occur in a moment in the externally called Fatalf
}
func (nl *nullLane) FatalfInternal(props loggingProperties, format string, args ...any) {
	nl.PreFatalfInternal(props, format, args...)
	// panic will occur in a moment in the externally called Fatalf
}

func (nl *nullLane) LogStackTrimInternal(props loggingProperties, message string, skippedCallers int) {
	nl.tee(nl.LaneProps(), func(teeProps loggingProperties, li laneInternal) {
		li.LogStackTrimInternal(teeProps, message, skippedCallers)
	})
}

func (nl *nullLane) OnPanic() {
	nl.onPanic()
}
