package lane

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"
)

type (
	// FilterFunc is a function that determines whether a log message should be passed through.
	// It receives the lane (for accessing journey ID, metadata, etc.), the log level,
	// and the formatted message. Returns true if the message should be logged, false otherwise.
	FilterFunc func(lane Lane, level LaneLogLevel, message string) bool

	filterLane struct {
		wrapped Lane
		filter  FilterFunc
	}
)

// NewFilterLane creates a new lane that wraps another lane and filters log messages
// based on the provided filter function. The filter receives the log level and formatted
// message, and should return true to allow the message through, false to block it.
//
// All non-logging methods (context operations, metadata, etc.) are passed through
// unchanged to the wrapped lane.
//
// For common use cases, see NewRegexFilterLane and NewLevelFilterLane.
func NewFilterLane(wrapped Lane, filter FilterFunc) Lane {
	if filter == nil {
		// No filter means pass everything through
		filter = func(lane Lane, level LaneLogLevel, message string) bool {
			return true
		}
	}

	return &filterLane{
		wrapped: wrapped,
		filter:  filter,
	}
}

// AddFilterTee adds a tee to the parent lane, optionally wrapping it with a regex filter.
// If pattern is empty, the tee is added directly without filtering.
// If pattern is provided, the tee is wrapped with NewRegexFilterLane before being added.
//
// Example:
//
//	mainLane := NewLogLane(nil)
//	AddFilterTee(mainLane, NewDiskLane(nil, "/var/log/audit.log"), `^\[AUDIT\]`)
//	AddFilterTee(mainLane, NewDiskLane(nil, "/var/log/all.log"), "")
func AddFilterTee(parent Lane, tee Lane, pattern string) {
	if pattern == "" {
		parent.AddTee(tee)
	} else {
		filteredTee := NewRegexFilterLane(tee, pattern)
		parent.AddTee(filteredTee)
	}
}

// NewRegexFilterLane creates a filtered lane that only passes messages matching the regex pattern.
//
// Example:
//
//	// Capture only [AUDIT] messages
//	auditLane := lane.NewRegexFilterLane(diskLane, `^\[AUDIT\]`)
//
//	// Capture messages containing "error" or "warning" (case-insensitive)
//	errorLane := lane.NewRegexFilterLane(diskLane, `(?i)(error|warning)`)
func NewRegexFilterLane(wrapped Lane, pattern string) Lane {
	re := regexp.MustCompile(pattern)
	filter := func(lane Lane, level LaneLogLevel, message string) bool {
		return re.MatchString(message)
	}
	return NewFilterLane(wrapped, filter)
}

// NewLevelFilterLane creates a filtered lane that only passes messages at or above the minimum level.
//
// Example:
//
//	// Only log warnings and errors
//	errorLane := lane.NewLevelFilterLane(diskLane, lane.LogLevelWarn)
func NewLevelFilterLane(wrapped Lane, minLevel LaneLogLevel) Lane {
	filter := func(lane Lane, level LaneLogLevel, message string) bool {
		return level >= minLevel
	}
	return NewFilterLane(wrapped, filter)
}

// NewRegexFilter creates a filter function that matches messages against a regular expression.
// Use this when you need to combine filters or use NewFilterLane directly.
func NewRegexFilter(pattern string) FilterFunc {
	re := regexp.MustCompile(pattern)
	return func(lane Lane, level LaneLogLevel, message string) bool {
		return re.MatchString(message)
	}
}

// NewLevelFilter creates a filter function based on minimum log level.
// Use this when you need to combine filters or use NewFilterLane directly.
func NewLevelFilter(minLevel LaneLogLevel) FilterFunc {
	return func(lane Lane, level LaneLogLevel, message string) bool {
		return level >= minLevel
	}
}

// Context methods - pass through to wrapped lane

func (fl *filterLane) Deadline() (deadline time.Time, ok bool) {
	return fl.wrapped.Deadline()
}

func (fl *filterLane) Done() <-chan struct{} {
	return fl.wrapped.Done()
}

func (fl *filterLane) Err() error {
	return fl.wrapped.Err()
}

func (fl *filterLane) Value(key any) any {
	return fl.wrapped.Value(key)
}

// Lane identity methods

func (fl *filterLane) LaneId() string {
	return fl.wrapped.LaneId()
}

func (fl *filterLane) JourneyId() string {
	return fl.wrapped.JourneyId()
}

func (fl *filterLane) SetJourneyId(id string) {
	fl.wrapped.SetJourneyId(id)
}

// Configuration methods

func (fl *filterLane) SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel) {
	return fl.wrapped.SetLogLevel(newLevel)
}

func (fl *filterLane) IsLevelEnabled(level LaneLogLevel) bool {
	return fl.wrapped.IsLevelEnabled(level)
}

func (fl *filterLane) SetMetadata(key, val string) {
	fl.wrapped.SetMetadata(key, val)
}

func (fl *filterLane) GetMetadata(key string) string {
	return fl.wrapped.GetMetadata(key)
}

// Logging methods - apply filter before forwarding

func (fl *filterLane) Trace(args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelTrace, msg) {
		fl.wrapped.Trace(args...)
	}
}

func (fl *filterLane) Tracef(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelTrace, msg) {
		fl.wrapped.Tracef(format, args...)
	}
}

func (fl *filterLane) TraceObject(message string, obj any) {
	if fl.filter(fl.wrapped, LogLevelTrace, message) {
		fl.wrapped.TraceObject(message, obj)
	}
}

func (fl *filterLane) Debug(args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelDebug, msg) {
		fl.wrapped.Debug(args...)
	}
}

func (fl *filterLane) Debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelDebug, msg) {
		fl.wrapped.Debugf(format, args...)
	}
}

func (fl *filterLane) DebugObject(message string, obj any) {
	if fl.filter(fl.wrapped, LogLevelDebug, message) {
		fl.wrapped.DebugObject(message, obj)
	}
}

func (fl *filterLane) Info(args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelInfo, msg) {
		fl.wrapped.Info(args...)
	}
}

func (fl *filterLane) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelInfo, msg) {
		fl.wrapped.Infof(format, args...)
	}
}

func (fl *filterLane) InfoObject(message string, obj any) {
	if fl.filter(fl.wrapped, LogLevelInfo, message) {
		fl.wrapped.InfoObject(message, obj)
	}
}

func (fl *filterLane) Warn(args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelWarn, msg) {
		fl.wrapped.Warn(args...)
	}
}

func (fl *filterLane) Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelWarn, msg) {
		fl.wrapped.Warnf(format, args...)
	}
}

func (fl *filterLane) WarnObject(message string, obj any) {
	if fl.filter(fl.wrapped, LogLevelWarn, message) {
		fl.wrapped.WarnObject(message, obj)
	}
}

func (fl *filterLane) Error(args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelError, msg) {
		fl.wrapped.Error(args...)
	}
}

func (fl *filterLane) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelError, msg) {
		fl.wrapped.Errorf(format, args...)
	}
}

func (fl *filterLane) ErrorObject(message string, obj any) {
	if fl.filter(fl.wrapped, LogLevelError, message) {
		fl.wrapped.ErrorObject(message, obj)
	}
}

func (fl *filterLane) PreFatal(args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, logLevelPreFatal, msg) {
		fl.wrapped.PreFatal(args...)
	}
}

func (fl *filterLane) PreFatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, logLevelPreFatal, msg) {
		fl.wrapped.PreFatalf(format, args...)
	}
}

func (fl *filterLane) PreFatalObject(message string, obj any) {
	if fl.filter(fl.wrapped, logLevelPreFatal, message) {
		fl.wrapped.PreFatalObject(message, obj)
	}
}

func (fl *filterLane) Fatal(args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelFatal, msg) {
		fl.wrapped.Fatal(args...)
	}
}

func (fl *filterLane) Fatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelFatal, msg) {
		fl.wrapped.Fatalf(format, args...)
	}
}

func (fl *filterLane) FatalObject(message string, obj any) {
	if fl.filter(fl.wrapped, LogLevelFatal, message) {
		fl.wrapped.FatalObject(message, obj)
	}
}

func (fl *filterLane) LogStack(message string) {
	if fl.filter(fl.wrapped, LogLevelStack, message) {
		fl.wrapped.LogStack(message)
	}
}

func (fl *filterLane) LogStackTrim(message string, skippedCallers int) {
	if fl.filter(fl.wrapped, LogLevelStack, message) {
		fl.wrapped.LogStackTrim(message, skippedCallers)
	}
}

// Utility methods

func (fl *filterLane) SetLengthConstraint(maxLength int) int {
	return fl.wrapped.SetLengthConstraint(maxLength)
}

func (fl *filterLane) Logger() *log.Logger {
	return fl.wrapped.Logger()
}

func (fl *filterLane) Close() {
	fl.wrapped.Close()
}

// Derive methods - wrap the derived lane in a new filter

func (fl *filterLane) Derive() Lane {
	derived := fl.wrapped.Derive()
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}
}

func (fl *filterLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	derived, cancel := fl.wrapped.DeriveWithCancel()
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}, cancel
}

func (fl *filterLane) DeriveWithCancelCause() (Lane, context.CancelCauseFunc) {
	derived, cancel := fl.wrapped.DeriveWithCancelCause()
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}, cancel
}

func (fl *filterLane) DeriveWithoutCancel() Lane {
	derived := fl.wrapped.DeriveWithoutCancel()
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}
}

func (fl *filterLane) DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc) {
	derived, cancel := fl.wrapped.DeriveWithDeadline(deadline)
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}, cancel
}

func (fl *filterLane) DeriveWithDeadlineCause(deadline time.Time, cause error) (Lane, context.CancelFunc) {
	derived, cancel := fl.wrapped.DeriveWithDeadlineCause(deadline, cause)
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}, cancel
}

func (fl *filterLane) DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc) {
	derived, cancel := fl.wrapped.DeriveWithTimeout(duration)
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}, cancel
}

func (fl *filterLane) DeriveWithTimeoutCause(duration time.Duration, cause error) (Lane, context.CancelFunc) {
	derived, cancel := fl.wrapped.DeriveWithTimeoutCause(duration, cause)
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}, cancel
}

func (fl *filterLane) DeriveReplaceContext(ctx OptionalContext) Lane {
	derived := fl.wrapped.DeriveReplaceContext(ctx)
	return &filterLane{
		wrapped: derived,
		filter:  fl.filter,
	}
}

// Stack trace and panic handling

func (fl *filterLane) EnableStackTrace(level LaneLogLevel, enable bool) (wasEnabled bool) {
	return fl.wrapped.EnableStackTrace(level, enable)
}

func (fl *filterLane) SetPanicHandler(handler Panic) {
	fl.wrapped.SetPanicHandler(handler)
}

// Tee methods

func (fl *filterLane) AddTee(l Lane) {
	fl.wrapped.AddTee(l)
}

func (fl *filterLane) RemoveTee(l Lane) {
	fl.wrapped.RemoveTee(l)
}

func (fl *filterLane) Tees() []Lane {
	return fl.wrapped.Tees()
}

// Parent access

func (fl *filterLane) Parent() Lane {
	parent := fl.wrapped.Parent()
	if parent == nil {
		return nil
	}
	// Wrap parent in filter too to maintain consistency
	return &filterLane{
		wrapped: parent,
		filter:  fl.filter,
	}
}

// laneInternal interface implementation - required for tee functionality

func (fl *filterLane) Constrain(msg string) string {
	if li, ok := fl.wrapped.(laneInternal); ok {
		return li.Constrain(msg)
	}
	return msg
}

func (fl *filterLane) LaneProps() loggingProperties {
	if li, ok := fl.wrapped.(laneInternal); ok {
		return li.LaneProps()
	}
	return loggingProperties{
		laneId:    fl.LaneId(),
		journeyId: fl.JourneyId(),
	}
}

func (fl *filterLane) TraceInternal(props loggingProperties, args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelTrace, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.TraceInternal(props, args...)
		}
	}
}

func (fl *filterLane) TracefInternal(props loggingProperties, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelTrace, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.TracefInternal(props, format, args...)
		}
	}
}

func (fl *filterLane) DebugInternal(props loggingProperties, args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelDebug, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.DebugInternal(props, args...)
		}
	}
}

func (fl *filterLane) DebugfInternal(props loggingProperties, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelDebug, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.DebugfInternal(props, format, args...)
		}
	}
}

func (fl *filterLane) InfoInternal(props loggingProperties, args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelInfo, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.InfoInternal(props, args...)
		}
	}
}

func (fl *filterLane) InfofInternal(props loggingProperties, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelInfo, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.InfofInternal(props, format, args...)
		}
	}
}

func (fl *filterLane) WarnInternal(props loggingProperties, args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelWarn, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.WarnInternal(props, args...)
		}
	}
}

func (fl *filterLane) WarnfInternal(props loggingProperties, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelWarn, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.WarnfInternal(props, format, args...)
		}
	}
}

func (fl *filterLane) ErrorInternal(props loggingProperties, args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelError, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.ErrorInternal(props, args...)
		}
	}
}

func (fl *filterLane) ErrorfInternal(props loggingProperties, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelError, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.ErrorfInternal(props, format, args...)
		}
	}
}

func (fl *filterLane) PreFatalInternal(props loggingProperties, args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, logLevelPreFatal, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.PreFatalInternal(props, args...)
		}
	}
}

func (fl *filterLane) PreFatalfInternal(props loggingProperties, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, logLevelPreFatal, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.PreFatalfInternal(props, format, args...)
		}
	}
}

func (fl *filterLane) FatalInternal(props loggingProperties, args ...any) {
	msg := sprint(args...)
	if fl.filter(fl.wrapped, LogLevelFatal, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.FatalInternal(props, args...)
		}
	}
}

func (fl *filterLane) FatalfInternal(props loggingProperties, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if fl.filter(fl.wrapped, LogLevelFatal, msg) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.FatalfInternal(props, format, args...)
		}
	}
}

func (fl *filterLane) LogStackTrimInternal(props loggingProperties, message string, skippedCallers int) {
	if fl.filter(fl.wrapped, LogLevelStack, message) {
		if li, ok := fl.wrapped.(laneInternal); ok {
			li.LogStackTrimInternal(props, message, skippedCallers)
		}
	}
}

func (fl *filterLane) OnPanic() {
	if li, ok := fl.wrapped.(laneInternal); ok {
		li.OnPanic()
	}
}
