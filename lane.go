package lane

import (
	"context"
	"log"
	"time"
)

const (
	LogLevelTrace LaneLogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
	logLevelPreFatal
	LogLevelStack
)

const logLevelMax = LogLevelStack + 1

type (
	LaneLogLevel int

	OptionalContext context.Context

	Lane interface {
		context.Context

		// Provides the correlation ID of the lane
		LaneId() string

		// Provides the journey ID (correlation across multiple processs/services/systems)
		JourneyId() string

		// Assigns an 'outer' correlation ID, intended for an end to end correlation that
		// may include an ID generated by some other part of the system.
		//
		// The ID will be truncated to 10 characters.
		//
		// Once set, log messages will include this ID along with the lane ID.
		SetJourneyId(id string)

		// Controls the log filtering
		SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel)

		// Sets a lane metadata value (even if the lane type does not log it)
		SetMetadata(key, val string)

		// Gets a lane metadata value (even if the lane type does not log it)
		GetMetadata(key string) string

		// Trace, intended for checkpoint information. Messages formated with fmt.Sprint().
		Trace(args ...any)
		// Trace, intended for checkpoint information. Messages formated with fmt.Sprintf().
		Tracef(format string, args ...any)
		// Trace, intended for checkpoint information. Object [obj] is converted to JSON, including private fields, and concatenated to [message].
		TraceObject(message string, obj any)

		// Debug, intended for diagnostic information such as unusual conditions or helpful variable values. Messages formated with fmt.Sprint().
		Debug(args ...any)
		// Debug, intended for diagnostic information such as unusual conditions or helpful variable values. Messages formated with fmt.Sprintf().
		Debugf(format string, args ...any)
		// Debug, intended for diagnostic information such as unusual conditions or helpful variable values. Object [obj] is converted to JSON, including private fields, and concatenated to [message].
		DebugObject(message string, obj any)

		// Info, intended for details as the app runs in a healthy state, such as end user requests and results. Messages formated with fmt.Sprint().
		Info(args ...any)
		// Info, intended for details as the app runs in a healthy state, such as end user requests and results. Messages formated with fmt.Sprintf().
		Infof(format string, args ...any)
		// Info, intended for details as the app runs in a healthy state, such as end user requests and results. Object [obj] is converted to JSON, including private fields, and concatenated to [message].
		InfoObject(message string, obj any)

		// Warn, intended for recoverable, ignorable or ambiguous errors. Messages formated with fmt.Sprint().
		Warn(args ...any)
		// Warn, intended for recoverable, ignorable or ambiguous errors. Messages formated with fmt.Sprintf().
		Warnf(format string, args ...any)
		// Warn, intended for recoverable, ignorable or ambiguous errors. Object [obj] is converted to JSON, including private fields, and concatenated to [message].
		WarnObject(message string, obj any)

		// Error, intended for application faults that alert or explain unwanted conditions. Messages formated with fmt.Sprint().
		Error(args ...any)
		// Error, intended for application faults that alert or explain unwanted conditions. Messages formated with fmt.Sprintf().
		Errorf(format string, args ...any)
		// Error, intended for application faults that alert or explain unwanted conditions. Object [obj] is converted to JSON, including private fields, and concatenated to [message].
		ErrorObject(message string, obj any)

		// Severe error, intended for details about why an application will soon terminate. Messages formated with fmt.Sprint().
		PreFatal(args ...any)
		// Severe error, intended for details about why an application will soon terminate. Messages formated with fmt.Sprintf().
		PreFatalf(format string, args ...any)
		// Severe error, intended for details about why an application will soon terminate. Object [obj] is converted to JSON, including private fields, and concatenated to [message].
		PreFatalObject(message string, obj any)

		// Fatal error, intended for details about why an application can't continue and must terminate. Messages formated with fmt.Sprint(). The app panics after logging completes.
		Fatal(args ...any)
		// Fatal error, intended for details about why an application can't continue and must terminate. Messages formated with fmt.Sprintf(). The app panics after logging completes.
		Fatalf(format string, args ...any)
		// Fatal error, intended for details about why an application can't continue and must terminate. Messages formated with fmt.Sprintf(). Object [obj] is converted to JSON, including private fields, and concatenated to [message].
		FatalObject(message string, obj any)

		// Logs the stack
		LogStack(message string)

		// Logs the stack, trimming the top of the stack by the number of [skippedCallers] specified
		LogStackTrim(message string, skippedCallers int)

		// Set a limit on the message length, or less than 1 for no limit.
		SetLengthConstraint(maxLength int) int

		// Exposes access to the underlying log object.
		Logger() *log.Logger
		Close()

		// Makes a lane for a child activity that needs its own correlation ID. For example a server will derive a new lane for each client connection.
		Derive() Lane

		// Makes a lane for a child activity that needs its own correlation ID, with the cancelable context.
		DeriveWithCancel() (Lane, context.CancelFunc)

		// Makes a lane for a child activity that needs its own correlation ID, with the cancelable context.
		// The cancel function can specify an error to indicate why the context was canceled.
		DeriveWithCancelCause() (Lane, context.CancelCauseFunc)

		// Makes a lane for a child activity that needs its own correlation ID, removing the cancelable context.
		DeriveWithoutCancel() Lane

		// Makes a lane for a child activity that needs its own correlation ID, with the time-canceled context.
		DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc)

		// Makes a lane for a child activity that needs its own correlation ID, with the time-canceled context.
		// The [cause] argument provides an error to use for timeout expiration.
		DeriveWithDeadlineCause(deadline time.Time, cause error) (Lane, context.CancelFunc)

		// Makes a lane for a child activity that needs its own correlation ID, with the relative time-canceled context.
		DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc)

		// Makes a lane for a child activity that needs its own correlation ID, with the relative time-canceled context.
		// The [cause] argument provides an error to use for timeout expiration.
		DeriveWithTimeoutCause(duration time.Duration, cause error) (Lane, context.CancelFunc)

		// Used to maintain the lane configuration while changing the context.
		DeriveReplaceContext(ctx OptionalContext) Lane

		// Turns on stack trace logging.
		EnableStackTrace(level LaneLogLevel, enable bool) (wasEnabled bool)

		// AddTee attaches a receiver lane to the sender lane. Log messages from the sender lane are
		// forwarded to the receiver lane [l], but retain the sender lane's lane ID and journey ID
		// instead of the receiver's IDs.
		AddTee(l Lane)

		// Disconnects the other lane from the tee.
		RemoveTee(l Lane)

		// Provides the current tee list
		Tees() []Lane

		// Intercepts Panic, allowing the test to prevent the executable from crashing, and validate
		// an injected fatal error. Use this with care, and be sure to call runtime.Goexit() so that
		// the test version of Panic doesn't return.
		SetPanicHandler(handler Panic)

		// Gets the parent lane, or untyped nil if no parent.
		Parent() Lane
	}

	Panic func()

	// functions for internal implementation
	laneInternal interface {
		Constrain(msg string) string

		LaneProps() loggingProperties

		TraceInternal(props loggingProperties, args ...any)
		TracefInternal(props loggingProperties, format string, args ...any)

		DebugInternal(props loggingProperties, args ...any)
		DebugfInternal(props loggingProperties, format string, args ...any)

		InfoInternal(props loggingProperties, args ...any)
		InfofInternal(props loggingProperties, format string, args ...any)

		WarnInternal(props loggingProperties, args ...any)
		WarnfInternal(props loggingProperties, format string, args ...any)

		ErrorInternal(props loggingProperties, args ...any)
		ErrorfInternal(props loggingProperties, format string, args ...any)

		PreFatalInternal(props loggingProperties, args ...any)
		PreFatalfInternal(props loggingProperties, format string, args ...any)

		FatalInternal(props loggingProperties, args ...any)
		FatalfInternal(props loggingProperties, format string, args ...any)

		LogStackTrimInternal(props loggingProperties, message string, skippedCallers int)
	}

	loggingProperties struct {
		laneId    string
		journeyId string
	}

	teeHandler func(props loggingProperties, receiver laneInternal)
)
