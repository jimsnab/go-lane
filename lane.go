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
)

type (
	LaneLogLevel int

	Lane interface {
		context.Context

		// Provides the correlation ID of the lane
		LaneId() string

		// Controls the log filtering
		SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel)

		// Trace, intended for checkpoint information. Messages formated with fmt.Sprint().
		Trace(args ...any)
		// Trace, intended for checkpoint information. Messages formated with fmt.Sprintf().
		Tracef(format string, args ...any)

		// Debug, intended for diagnostic information such as unusual conditions or helpful variable values. Messages formated with fmt.Sprint().
		Debug(args ...any)
		// Debug, intended for diagnostic information such as unusual conditions or helpful variable values. Messages formated with fmt.Sprintf().
		Debugf(format string, args ...any)

		// Info, intended for details as the app runs in a healthy state, such as end user requests and results. Messages formated with fmt.Sprint().
		Info(args ...any)
		// Info, intended for details as the app runs in a healthy state, such as end user requests and results. Messages formated with fmt.Sprintf().
		Infof(format string, args ...any)

		// Warn, intended for recoverable, ignorable or ambiguous errors. Messages formated with fmt.Sprint().
		Warn(args ...any)
		// Warn, intended for recoverable, ignorable or ambiguous errors. Messages formated with fmt.Sprintf().
		Warnf(format string, args ...any)

		// Error, intended for application faults that alert or explain unwanted conditions. Messages formated with fmt.Sprint().
		Error(args ...any)
		// Error, intended for application faults that alert or explain unwanted conditions. Messages formated with fmt.Sprintf().
		Errorf(format string, args ...any)

		// Severe error, intended for details about why an application will soon terminate. Messages formated with fmt.Sprint().
		PreFatal(args ...any)
		// Severe error, intended for details about why an application will soon terminate. Messages formated with fmt.Sprintf().
		PreFatalf(format string, args ...any)

		// Fatal error, intended for details about why an application can't continue and must terminate. Messages formated with fmt.Sprint(). The app panics after logging completes.
		Fatal(args ...any)
		// Fatal error, intended for details about why an application can't continue and must terminate. Messages formated with fmt.Sprintf(). The app panics after logging completes.
		Fatalf(format string, args ...any)

		// Exposes access to the underlying log object.
		Logger() *log.Logger
		Close()

		// Makes a lane for a child activity that needs its own correlation ID. For example a server will derive a new lane for each client connection.
		Derive() Lane

		// Makes a lane for a child activity that needs its own correlation ID, with the cancelable context.
		DeriveWithCancel() (Lane, context.CancelFunc)

		// Makes a lane for a child activity that needs its own correlation ID, with the time-canceled context.
		DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc)

		// Makes a lane for a child activity that needs its own correlation ID, with the relative time-canceled context.
		DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc)

		// Used to maintain the lane configuration while changing the context.
		DeriveReplaceContext(ctx context.Context) Lane

		// Turns on stack trace logging.
		EnableStackTrace(level LaneLogLevel, enable bool) (wasEnabled bool)

		// Replicates the logging activity in another lane.
		AddTee(l Lane)

		// Disconnects the other lane from the tee.
		RemoveTee(l Lane)
	}
)
