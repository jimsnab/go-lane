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

	"github.com/google/uuid"
)

type (
	LogLane interface {
		Lane
		AddCR(shouldAdd bool) (prior bool)
	}

	logLane struct {
		context.Context
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
		onCreateLane OnCreateLane
	}

	wrappedLogWriter struct {
		ll *logLane
	}

	laneId string

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
const LogLaneIdKey = laneId("log_lane_id")

// Context key for the parent lane ID
const ParentLaneIdKey = laneId("parent_lane_id")

func isLogCrLf() bool {
	var buf bytes.Buffer
	testLog := log.New(&buf, "", 0)
	testLog.Println()
	return buf.Bytes()[0] == '\r'
}

func NewLogLane(ctx context.Context) Lane {
	l, _ := deriveLogLane(nil, ctx, nil, createLogLane)
	return l
}

// Initializes a LogLane for a more sophisticated lane type that embeds a log lane.
//
//   - onCreate creates a new instance of the outer lane and provides the embedded log lane.
//   - startingCtx provides an optional context instance, to start a lane from a pre-existing
//     context
func NewEmbeddedLogLane(onCreate OnCreateLane, startingCtx context.Context) (l Lane, err error) {
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
	newLane = &llZero
	ll = &llZero
	return
}

// Function to allocate a log lane for lane types that embed a log lane
func AllocEmbeddedLogLane() LogLane {
	llZero := logLane{}
	return &llZero
}

// Convenience wrapper that makes a new logLane object and calls initialize() on it
func deriveLogLane(parent *logLane, startingCtx context.Context, contextCallback deriveContext, createLane OnCreateLane) (ll *logLane, err error) {
	var parentOuter Lane
	if parent != nil {
		parentOuter = parent.outer
	}
	childOuter, child, writer, err := createLane(parentOuter)
	if err != nil {
		return
	}
	ll = child.(*logLane)
	ll.initialize(childOuter, parent, startingCtx, contextCallback, createLane, writer)
	return
}

// Sets all the fields of a zero-initialized ll
func (ll *logLane) initialize(laneOuter Lane, pll *logLane, startingCtx context.Context, contextCallback deriveContext, onCreate OnCreateLane, writer *log.Logger) {
	ll.stackTrace = make([]atomic.Bool, int(LogLevelFatal+1))
	ll.onCreateLane = onCreate // keep this reference so that future Derive() calls can invoke it
	ll.SetPanicHandler(nil)

	// make a logging instance that ultimately does logging via the lane
	wlw := wrappedLogWriter{ll: ll}
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
	} else {
		ll.wlog.SetFlags(log.LstdFlags)
		ll.tees = []Lane{}
		ll.cr = ""
	}

	id := uuid.New().String()
	id = id[len(id)-10:]

	// The context must have the correlation ID value set. The caller might also
	// want another context feature such as WithCancel or WithDeadline. This requires
	// conditional wrapping, which is supported here with an optional callback.
	var newCtx context.Context
	if startingCtx == nil {
		startingCtx = context.Background()
	}

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
func NewLogLaneWithCR(ctx context.Context) Lane {
	cr := ""
	if !isLogCrLf() {
		cr = "\r"
	}

	ll, _ := deriveLogLane(nil, ctx, nil, createLogLane)
	ll.cr = cr
	return ll
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
		ll.writer.SetFlags(ll.wlog.Flags() &^ ll.logMask)
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
	ll.onPanic()
}

func (ll *logLane) Fatalf(format string, args ...any) {
	ll.PreFatalf(format, args...)
	ll.onPanic()
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

func (ll *logLane) DeriveReplaceContext(ctx context.Context) Lane {
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

func (ll *logLane) SetPanicHandler(handler Panic) {
	ll.mu.Lock()
	defer ll.mu.Unlock()

	if handler == nil {
		handler = func() { panic("fatal error") }
	}
	ll.onPanic = handler
}

func (ll *logLane) setFlagsMask(mask int) {
	ll.logMask = mask
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
	wlw.ll.Info(text)

	return len(p), nil
}
