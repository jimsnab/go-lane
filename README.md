# go-lane

A "lane" is a context that has logging associated. It is a melding of Go's `log` and its `context`.

# Basic Use

```go
import (
    "context"
    "github.com/jimsnab/go-lane"
)

func myFunc() {
    l := lane.NewLogLane(context.Background())

    l.Info("log something")
}
```

At the root, a lane needs a context, and that is typically `context.Background()`. From there, instead of
passing a `context` instance as the first parameter, pass the lane `l`.

```go
func someFunc(l lane.Lane) {
     // use l like a context instance, or call one of its interface members
}
```

# Interface
```go
Lane interface {
	context.Context
	LaneId() string
	SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel)
	Trace(args ...any)
	Tracef(format string, args ...any)
	Debug(args ...any)
	Debugf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Logger() *log.Logger
	Close()

	Derive() Lane
	DeriveWithCancel() (Lane, context.CancelFunc)
	DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc)
	DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc)
	DeriveReplaceContext(ctx context.Context) Lane

	EnableStackTrace(level LaneLogLevel, enable bool) (wasEnabled bool)
}
```

For the most part, application code will use the logging functions (Trace, Debug, ...).

A correlation ID is provided via `LaneId()`, which is automatically inserted into the
logged message.

When spawining go routines, pass `l` around, or use one of the Derive functions when
a new correlation ID is needed.

# Types of Lanes
* `NewLogLane` log messages go to the standard Go `log` infrastructure. Access the `log`
  instance via `Logger()` to set flags, add a prefix, or change output I/O.
* `NewDiskLane` like a "log lane" but writes output to a file.
* `NewTestingLane` captures log messages into a buffer and provides `VerifyEvents()`,
  `VerifyEventText()` and `EventsToString()` for use in unit test code that checks the log to confirm
  an expected result.
* `NewNullLane` creates a lane that does not log but still has the context functionality.
  Logging is similar to `log.SetOutput(io.Discard)` - fatal errors still terminate the app.

Normally the production code uses a log lane, and unit tests use a testing lane; a null
lane is handy in unit tests to disable logging out of scope of the test.

The code doing the logging or using the context should not care what kind of lane it
is given to use.

# Stack Trace
Stack trace logging can be enabled on a per level basis. For example, to enable stack
trace output for `ERROR`:

```go
func example() {
	l := NewLogLane(context.Background())
	l.EnableStackTrace(lane.LogLevelError, true)
	l.Error("sample")   // stack trace is logged also
}
```
