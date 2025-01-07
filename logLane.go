package lane

import (
	"bufio"
	"bytes"
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
	LogLane interface {
		Lane
		laneInternal
		AddCR(shouldAdd bool) (prior bool)
		SetFlagsMask(mask int) (prior int)
	}

	logLane struct {
		context.Context
		MetadataStore
		wlog         *log.Logger // wrapper log to capture caller's logging intent without sending to output
		writer       *log.Logger // the log instance used for output
		level        int32
		cr           string
		stackTrace   []atomic.Bool
		mu           sync.Mutex
		tees         []Lane
		journeyId    string
		onPanic      Panic
		logMask      int
		outer        Lane
		parent       *logLane
		onCreateLane OnCreateLane
		maxLength    atomic.Int32
	}

	wrappedLogWriter struct {
		outer Lane
		ll    *logLane
	}

	LaneIdKey string

	// Callback for creating a new derived context. If the context returned by
	// the callback is not derived from newCtx, the returned context must
	// contain the context value key laneIdKey with value id.
	deriveContext func(newCtx context.Context, id string) context.Context

	// Callback invoked when a derived context is created. It is used by log
	// lane types that embed a log lane. It allows the outer lane type to
	// make its lane-specific object. The callback must provide newLane and ll.
	// Returning non-nil writer is optional.
	OnCreateLane func(parentLane Lane) (newLane Lane, ll LogLane, writer *log.Logger, err error)
)

// Context key for the lane ID
const LogLaneIdKey = LaneIdKey("log_lane_id")

// Context key for the parent lane ID
const ParentLaneIdKey = LaneIdKey("parent_lane_id")

func isLogCrLf() bool {
	var buf bytes.Buffer
	testLog := log.New(&buf, "", 0)
	testLog.Println()
	return buf.Bytes()[0] == '\r'
}

func NewLogLane(ctx OptionalContext) Lane {
	l, _ := deriveLogLane(nil, ctx, nil, createLogLane)
	return l
}

// Initializes a LogLane for a more sophisticated lane type that embeds a log lane.
//
//   - onCreate creates a new instance of the outer lane and provides the embedded log lane.
//   - startingCtx provides an optional context instance, to start a lane from a pre-existing
//     context
func NewEmbeddedLogLane(onCreate OnCreateLane, startingCtx OptionalContext) (l Lane, err error) {
	laneOuter, embedded, writer, err := onCreate(nil)
	if err != nil {
		return
	}

	ll := embedded.(*logLane)
	ll.initialize(laneOuter, nil, startingCtx, nil, onCreate, writer)
	l = laneOuter
	return
}

// Worker that makes an uninitialized log lane
func createLogLane(parentLane Lane) (newLane Lane, ll LogLane, writer *log.Logger, err error) {
	llZero := logLane{}
	llZero.SetOwner(&llZero)
	newLane = &llZero
	ll = &llZero
	return
}

// Function to allocate a log lane for lane types that embed a log lane
func AllocEmbeddedLogLane() LogLane {
	llZero := logLane{}
	llZero.SetOwner(&llZero)
	return &llZero
}

// Convenience wrapper that makes a new logLane object and calls initialize() on it
func deriveLogLane(parent *logLane, startingCtx context.Context, contextCallback deriveContext, createLane OnCreateLane) (l Lane, err error) {
	var parentOuter Lane
	if parent != nil {
		parentOuter = parent.outer
	}
	childOuter, child, writer, err := createLane(parentOuter)
	if err != nil {
		return
	}
	derived := child.(*logLane)
	derived.initialize(childOuter, parent, startingCtx, contextCallback, createLane, writer)

	l = childOuter
	return
}

// Sets all the fields of a zero-initialized ll
func (ll *logLane) initialize(laneOuter Lane, pll *logLane, startingCtx context.Context, contextCallback deriveContext, onCreate OnCreateLane, writer *log.Logger) {
	if startingCtx == nil {
		startingCtx = context.Background()
	}

	ll.stackTrace = make([]atomic.Bool, int(LogLevelStack+1))
	ll.EnableStackTrace(LogLevelStack, true)
	ll.onCreateLane = onCreate // keep this reference so that future Derive() calls can invoke it
	ll.outer = laneOuter
	ll.parent = pll
	ll.SetPanicHandler(nil)

	// make a logging instance that ultimately does logging via the lane
	wlw := wrappedLogWriter{outer: laneOuter, ll: ll}
	if writer == nil {
		ll.writer = log.Default()
	} else {
		ll.writer = writer
	}
	ll.wlog = log.New(&wlw, "", 0)

	if pll != nil {
		ll.journeyId = pll.journeyId
		ll.tees = pll.tees
		ll.cr = pll.cr
		ll.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&pll.level)))
		ll.wlog.SetFlags(pll.wlog.Flags())
		ll.wlog.SetPrefix(pll.wlog.Prefix())
		ll.onPanic = pll.onPanic
		copyConfigToDerivation(ll, pll)
	} else {
		ll.wlog.SetFlags(log.LstdFlags)
		ll.tees = []Lane{}
		ll.cr = ""
	}

	id := makeLaneId()

	// The context must have the correlation ID value set. The caller might also
	// want another context feature such as WithCancel or WithDeadline. This requires
	// conditional wrapping, which is supported here with an optional callback.
	var newCtx context.Context

	if pll != nil {
		newCtx = context.WithValue(context.WithValue(startingCtx, LogLaneIdKey, id), ParentLaneIdKey, pll.LaneId())
	} else {
		newCtx = context.WithValue(startingCtx, LogLaneIdKey, id)
	}
	if contextCallback != nil {
		ll.Context = contextCallback(newCtx, id)
	} else {
		ll.Context = newCtx
	}
}

func (ll *logLane) AddCR(shouldAdd bool) (prior bool) {
	ll.mu.Lock()
	prior = (ll.cr != "")
	if shouldAdd {
		ll.cr = "\r"
	} else {
		ll.cr = ""
	}
	ll.mu.Unlock()
	return
}

// For cases where \r\n line endings are required (ex: vscode terminal)
func NewLogLaneWithCR(ctx OptionalContext) Lane {
	ll, _ := deriveLogLane(nil, ctx, nil, createLogLane)
	if !isLogCrLf() {
		p := ll.(LogLane)
		p.AddCR(true)
	}
	return ll
}

// Adds an ID to the log message(s)
func (ll *logLane) SetJourneyId(id string) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

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

func (ll *logLane) shouldLog(level LaneLogLevel) bool {
	if atomic.LoadInt32(&ll.level) <= int32(level) {
		// the log wrapper is exposed to the client, so ensure changes
		// made to prefix and flags are copied into the instance
		// generating the output
		ll.writer.SetPrefix(ll.wlog.Prefix())
		ll.writer.SetFlags(ll.wlog.Flags() &^ ll.logMask)
		return true
	}

	return false
}

func (ll *logLane) tee(props loggingProperties, logger teeHandler) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	for _, t := range ll.tees {
		receiver := t.(laneInternal)
		logger(props, receiver)
	}
}

func (ll *logLane) printMsg(props loggingProperties, level LaneLogLevel, prefix string, teeFn teeHandler, args ...any) {
	if ll.shouldLog(level) {
		msg := fmt.Sprintf("%s %s", props.getMessagePrefix(prefix), sprint(args...))
		if ll.cr != "" {
			msg = strings.ReplaceAll(msg, "\r\n", "\n")
			msg = strings.ReplaceAll(msg, "\n", ll.cr+"\n")
			if !strings.Contains(msg, ll.cr) {
				msg += ll.cr
			}
		}
		ll.writer.Print(msg)
		ll.logStackIf(props, level, "", 0)
	}
	ll.tee(props, teeFn)
}

func (ll *logLane) Constrain(text string) string {
	maxLen := ll.maxLength.Load()
	if maxLen > 0 && len(text) > int(maxLen) {
		text = text[:maxLen-1] + "\u2026"
	}
	return text
}

func (ll *logLane) printfMsg(props loggingProperties, level LaneLogLevel, prefix string, teeFn teeHandler, formatStr string, args ...any) {
	if ll.shouldLog(level) {
		text := ll.Constrain(fmt.Sprintf(formatStr, args...))

		msg := fmt.Sprintf("%s %s", props.getMessagePrefix(prefix), text)
		if ll.cr != "" {
			msg = strings.ReplaceAll(msg, "\r\n", "\n")
			msg = strings.ReplaceAll(msg, "\n", ll.cr+"\n")
			if !strings.Contains(msg, ll.cr) {
				msg += ll.cr
			}
		}
		ll.writer.Print(msg)
		ll.logStackIf(props, level, "", 0)
	}
	ll.tee(props, teeFn)
}

func (ll *logLane) LaneProps() loggingProperties {
	ll.mu.Lock()
	defer ll.mu.Unlock()
	return loggingProperties{
		laneId:    ll.LaneId(),
		journeyId: ll.journeyId,
	}
}

func (ll *logLane) Trace(args ...any) {
	ll.TraceInternal(ll.LaneProps(), args...)
}

func (ll *logLane) Tracef(format string, args ...any) {
	ll.TracefInternal(ll.LaneProps(), format, args...)
}

func (ll *logLane) TraceObject(message string, obj any) {
	LogObject(ll, LogLevelTrace, message, obj)
}

func (ll *logLane) Debug(args ...any) {
	ll.DebugInternal(ll.LaneProps(), args...)
}

func (ll *logLane) Debugf(format string, args ...any) {
	ll.DebugfInternal(ll.LaneProps(), format, args...)
}

func (ll *logLane) DebugObject(message string, obj any) {
	LogObject(ll, LogLevelDebug, message, obj)
}

func (ll *logLane) Info(args ...any) {
	ll.InfoInternal(ll.LaneProps(), args...)
}

func (ll *logLane) Infof(format string, args ...any) {
	ll.InfofInternal(ll.LaneProps(), format, args...)
}

func (ll *logLane) InfoObject(message string, obj any) {
	LogObject(ll, LogLevelInfo, message, obj)
}

func (ll *logLane) Warn(args ...any) {
	ll.WarnInternal(ll.LaneProps(), args...)
}

func (ll *logLane) Warnf(format string, args ...any) {
	ll.WarnfInternal(ll.LaneProps(), format, args...)
}

func (ll *logLane) WarnObject(message string, obj any) {
	LogObject(ll, LogLevelWarn, message, obj)
}

func (ll *logLane) Error(args ...any) {
	ll.ErrorInternal(ll.LaneProps(), args...)
}

func (ll *logLane) Errorf(format string, args ...any) {
	ll.ErrorfInternal(ll.LaneProps(), format, args...)
}

func (ll *logLane) ErrorObject(message string, obj any) {
	LogObject(ll, LogLevelError, message, obj)
}

func (ll *logLane) PreFatal(args ...any) {
	ll.PreFatalInternal(ll.LaneProps(), args...)
}

func (ll *logLane) PreFatalf(format string, args ...any) {
	ll.PreFatalfInternal(ll.LaneProps(), format, args...)
}

func (ll *logLane) PreFatalObject(message string, obj any) {
	LogObject(ll, logLevelPreFatal, message, obj)
}

func (ll *logLane) Fatal(args ...any) {
	ll.FatalInternal(ll.LaneProps(), args...)
	ll.onPanic()
}

func (ll *logLane) Fatalf(format string, args ...any) {
	ll.FatalfInternal(ll.LaneProps(), format, args...)
	ll.onPanic()
}

func (ll *logLane) FatalObject(message string, obj any) {
	ll.PreFatalObject(message, obj)
	ll.onPanic()
}

func (ll *logLane) logStackIf(props loggingProperties, level LaneLogLevel, message string, skipCallers int) {
	if ll.stackTrace[level].Load() && level != LogLevelStack {
		ll.logStack(props, message, skipCallers)
	}
}

func (ll *logLane) logStack(props loggingProperties, message string, skipCallers int) {
	buf := make([]byte, 16384)
	n := runtime.Stack(buf, false)
	lines := cleanStack(buf[:n], skipCallers)

	if message != "" {
		ll.writer.Printf("%s %s%s", props.getMessagePrefix("STACK"), ll.Constrain(message), ll.cr)
	}

	// each has two lines (the function name on one line, followed by source info on the next line)
	for _, line := range lines {
		ll.writer.Printf("%s %s%s", props.getMessagePrefix("STACK"), ll.Constrain(line), ll.cr)
	}
}

func (ll *logLane) LogStack(message string) {
	ll.LogStackTrim(message, 0)
}

func (ll *logLane) LogStackTrim(message string, skippedCallers int) {
	ll.LogStackTrimInternal(ll.LaneProps(), message, skippedCallers)
}

func (ll *logLane) SetLengthConstraint(maxLength int) int {
	old := ll.maxLength.Load()
	if maxLength > 1 {
		ll.maxLength.Store(int32(maxLength))
	} else {
		ll.maxLength.Store(0)
	}
	return int(old)
}

func (ll *logLane) Logger() *log.Logger {
	return ll.wlog
}

func (ll *logLane) Close() {
}

func (ll *logLane) Derive() Lane {
	l, err := deriveLogLane(ll, ll, nil, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l
}

func (ll *logLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	var cancelFn context.CancelFunc
	makeContext := func(newCtx context.Context, id string) context.Context {
		var childCtx context.Context
		childCtx, cancelFn = context.WithCancel(newCtx)
		return childCtx
	}
	l, err := deriveLogLane(ll, ll, makeContext, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l, cancelFn
}

func (ll *logLane) DeriveWithCancelCause() (Lane, context.CancelCauseFunc) {
	var cancelFn context.CancelCauseFunc
	makeContext := func(newCtx context.Context, id string) context.Context {
		var childCtx context.Context
		childCtx, cancelFn = context.WithCancelCause(newCtx)
		return childCtx
	}
	l, err := deriveLogLane(ll, ll, makeContext, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l, cancelFn
}

func (ll *logLane) DeriveWithoutCancel() Lane {
	makeContext := func(newCtx context.Context, id string) context.Context {
		return context.WithoutCancel(newCtx)
	}
	l, err := deriveLogLane(ll, ll, makeContext, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l
}

func (ll *logLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	var cancelFn context.CancelFunc
	makeContext := func(newCtx context.Context, id string) context.Context {
		var childCtx context.Context
		childCtx, cancelFn = context.WithDeadline(newCtx, deadline)
		return childCtx
	}
	l, err := deriveLogLane(ll, ll, makeContext, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l, cancelFn
}

func (ll *logLane) DeriveWithDeadlineCause(deadline time.Time, cause error) (Lane, context.CancelFunc) {
	var cancelFn context.CancelFunc
	makeContext := func(newCtx context.Context, id string) context.Context {
		var childCtx context.Context
		childCtx, cancelFn = context.WithDeadlineCause(newCtx, deadline, cause)
		return childCtx
	}
	l, err := deriveLogLane(ll, ll, makeContext, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l, cancelFn
}

func (ll *logLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	var cancelFn context.CancelFunc
	makeContext := func(newCtx context.Context, id string) context.Context {
		var childCtx context.Context
		childCtx, cancelFn = context.WithTimeout(newCtx, duration)
		return childCtx
	}
	l, err := deriveLogLane(ll, ll, makeContext, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l, cancelFn
}

func (ll *logLane) DeriveWithTimeoutCause(duration time.Duration, cause error) (Lane, context.CancelFunc) {
	var cancelFn context.CancelFunc
	makeContext := func(newCtx context.Context, id string) context.Context {
		var childCtx context.Context
		childCtx, cancelFn = context.WithTimeoutCause(newCtx, duration, cause)
		return childCtx
	}
	l, err := deriveLogLane(ll, ll, makeContext, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l, cancelFn
}

func (ll *logLane) DeriveReplaceContext(ctx OptionalContext) Lane {
	if ctx == nil {
		ctx = context.Background()
	}

	makeContext := func(newCtx context.Context, id string) context.Context {
		return context.WithValue(ctx, LogLaneIdKey, id)
	}
	l, err := deriveLogLane(ll, ctx, makeContext, ll.onCreateLane)
	if err != nil {
		l.Fatal(err)
	}
	return l
}

func (ll *logLane) LaneId() string {
	return ll.Value(LogLaneIdKey).(string)
}

func (ll *logLane) JourneyId() string {
	ll.mu.Lock()
	defer ll.mu.Unlock()
	return ll.journeyId
}

func (ll *logLane) EnableStackTrace(level LaneLogLevel, enable bool) bool {
	return ll.stackTrace[level].Swap(enable)
}

func (ll *logLane) AddTee(l Lane) {
	ll.mu.Lock()
	for _, t := range ll.tees {
		if t.LaneId() == l.LaneId() {
			// can't create a cyclical tee
			panic("tee points to itself")
		}
	}
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

func (ll *logLane) Tees() []Lane {
	ll.mu.Lock()
	defer ll.mu.Unlock()
	tees := make([]Lane, len(ll.tees))
	copy(tees, ll.tees)
	return tees
}

func (ll *logLane) SetPanicHandler(handler Panic) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	if handler == nil {
		handler = func() { panic("fatal error") }
	}
	ll.onPanic = handler
}

func (ll *logLane) SetFlagsMask(mask int) (prior int) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	prior = ll.logMask
	ll.logMask = mask
	return
}

func (wlw *wrappedLogWriter) Write(p []byte) (n int, err error) {
	text := string(p)

	// The wrapped logger has already written some prefix text, which
	// is out of our control.
	//
	// Make a temporary log to re-create the prefix without any message,
	// so it be stripped and duplicate prefix is prevented.
	var prefix bytes.Buffer
	w := bufio.NewWriter(&prefix)
	sublog := log.New(w, wlw.ll.wlog.Prefix(), wlw.ll.wlog.Flags())
	sublog.Print()
	w.Flush()

	prefixText := strings.TrimSpace(prefix.String())
	if prefixText != "" {
		parts := strings.Split(prefixText, " ")
		cuts := len(parts)
		for cuts > 0 {
			cutPoint := strings.Index(text, " ")
			if cutPoint < 0 {
				break
			}
			text = text[cutPoint+1:]
			cuts--
		}
	}
	wlw.outer.Info(text)

	return len(p), nil
}

func (ll *logLane) Parent() Lane {
	if ll.parent != nil {
		return ll.parent
	}
	return nil // untyped nil
}

func (ll *logLane) TraceInternal(props loggingProperties, args ...any) {
	ll.printMsg(props, LogLevelTrace, "TRACE", func(teeProps loggingProperties, li laneInternal) { li.TraceInternal(teeProps, args...) }, args...)
}

func (ll *logLane) TracefInternal(props loggingProperties, format string, args ...any) {
	ll.printfMsg(props, LogLevelTrace, "TRACE", func(teeProps loggingProperties, li laneInternal) { li.TracefInternal(teeProps, format, args...) }, format, args...)
}

func (ll *logLane) DebugInternal(props loggingProperties, args ...any) {
	ll.printMsg(props, LogLevelDebug, "DEBUG", func(teeProps loggingProperties, li laneInternal) { li.DebugInternal(teeProps, args...) }, args...)
}

func (ll *logLane) DebugfInternal(props loggingProperties, format string, args ...any) {
	ll.printfMsg(props, LogLevelDebug, "DEBUG", func(teeProps loggingProperties, li laneInternal) { li.DebugfInternal(teeProps, format, args...) }, format, args...)
}

func (ll *logLane) InfoInternal(props loggingProperties, args ...any) {
	ll.printMsg(props, LogLevelInfo, "INFO", func(teeProps loggingProperties, li laneInternal) { li.InfoInternal(teeProps, args...) }, args...)
}

func (ll *logLane) InfofInternal(props loggingProperties, format string, args ...any) {
	ll.printfMsg(props, LogLevelInfo, "INFO", func(teeProps loggingProperties, li laneInternal) { li.InfofInternal(teeProps, format, args...) }, format, args...)
}

func (ll *logLane) WarnInternal(props loggingProperties, args ...any) {
	ll.printMsg(props, LogLevelWarn, "WARN", func(teeProps loggingProperties, li laneInternal) { li.WarnInternal(teeProps, args...) }, args...)
}

func (ll *logLane) WarnfInternal(props loggingProperties, format string, args ...any) {
	ll.printfMsg(props, LogLevelWarn, "WARN", func(teeProps loggingProperties, li laneInternal) { li.WarnfInternal(teeProps, format, args...) }, format, args...)
}

func (ll *logLane) ErrorInternal(props loggingProperties, args ...any) {
	ll.printMsg(props, LogLevelError, "ERROR", func(teeProps loggingProperties, li laneInternal) { li.ErrorInternal(teeProps, args...) }, args...)
}

func (ll *logLane) ErrorfInternal(props loggingProperties, format string, args ...any) {
	ll.printfMsg(props, LogLevelError, "ERROR", func(teeProps loggingProperties, li laneInternal) { li.ErrorfInternal(teeProps, format, args...) }, format, args...)
}

func (ll *logLane) PreFatalInternal(props loggingProperties, args ...any) {
	ll.printMsg(ll.LaneProps(), LogLevelFatal, "FATAL", func(teeProps loggingProperties, li laneInternal) { li.PreFatalInternal(teeProps, args...) }, args...)
}

func (ll *logLane) PreFatalfInternal(props loggingProperties, format string, args ...any) {
	ll.printfMsg(ll.LaneProps(), LogLevelFatal, "FATAL", func(teeProps loggingProperties, li laneInternal) { li.PreFatalfInternal(teeProps, format, args...) }, format, args...)
}

func (ll *logLane) FatalInternal(props loggingProperties, args ...any) {
	ll.PreFatalInternal(props, args...)
	// panic will happen in a moment on the externally called Fatal()
}

func (ll *logLane) FatalfInternal(props loggingProperties, format string, args ...any) {
	ll.PreFatalfInternal(props, format, args...)
	// panic will happen in a moment on the externally called Fatalf()
}

func (ll *logLane) LogStackTrimInternal(props loggingProperties, message string, skippedCallers int) {
	if ll.shouldLog(LogLevelStack) {
		ll.logStack(props, message, skippedCallers)
	}
	ll.tee(props, func(teeProps loggingProperties, li laneInternal) {
		li.LogStackTrimInternal(teeProps, message, skippedCallers)
	})
}

func (ll *logLane) OnPanic() {
	ll.onPanic()
}
