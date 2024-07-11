# go-lane

A "lane" is a context that has logging associated. It is a melding of Go's `log` and its `context`.

A lane is intended to be passed into everything, and used like a regular `context` for patterns such as cancelation, and also used for logging in place of `log`.

The logging interface provides several tools for rich diagnostics, including 
correlation IDs, stack traces, easy logging of structs including private members, 
log capturing for test verification, recording data differences between two structs, 
and more.

Sister projects adapt logs to unify them into one logging solution, such as [go-lane-gin](https://github.com/jimsnab/go-lane-gin) and
[go-lane-opensearch](https://github.com/jimsnab/go-lane-opensearch).

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

At the root, a lane needs a context, and that is typically `nil` or `context.Background()`. From there, 
instead of passing a `context` instance as the first parameter, pass the lane `l`.

```go
func someFunc(l lane.Lane) {
     // use l like a context instance, or call one of its interface members
}
```

# Interface

## Lane Instance
```go
Lane interface {
	context.Context
	LaneId() string
	SetJourneyId(id string)
	SetLogLevel(newLevel LaneLogLevel) (priorLevel LaneLogLevel)

	Trace(args ...any)
	Tracef(format string, args ...any)
	TraceObject(message string, obj any)

	Debug(args ...any)
	Debugf(format string, args ...any)
	DebugObject(message string, obj any)

	Info(args ...any)
	Infof(format string, args ...any)
	InfoObject(message string, obj any)

	Warn(args ...any)
	Warnf(format string, args ...any)
	WarnObject(message string, obj any)

	Error(args ...any)
	Errorf(format string, args ...any)
	ErrorObject(message string, obj any)

	PreFatal(args ...any)
	PreFatalf(format string, args ...any)
	PreFatalObject(message string, obj any)

	Fatal(args ...any)
	Fatalf(format string, args ...any)
	FatalObject(message string, obj any)

	Logger() *log.Logger
	Close()

	Derive() Lane

	DeriveWithCancel() (Lane, context.CancelFunc)
	DeriveWithCancelCause() (Lane, context.CancelCauseFunc)
	DeriveWithoutCancel() (Lane, context.CancelFunc)

	DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc)
	DeriveWithDeadlineCause(deadline time.Time, cause error) (Lane, context.CancelFunc)

	DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc)
	DeriveWithTimeoutCause(duration time.Duration, cause error) (Lane, context.CancelFunc)
	
	DeriveReplaceContext(ctx OptionalContext) Lane

	EnableStackTrace(level LaneLogLevel, enable bool) (wasEnabled bool)

	AddTee(l Lane)
	RemoveTee(l Lane)

	SetPanicHandler(handler Panic)

	Parent() Lane
}
```

For the most part, application code will use the logging functions (`Trace`, `Debug`, ...).

Each log level provides:

* a `Sprint` version (ex: `Info` or `Error`)
* a `Sprintf` version (ex: `Infof` or `Errorf`)
* an object logger (ex: `InfoObject` or `ErrorObject`)

The object logger will convert an object into JSON, including private fields.

A correlation ID is provided via `LaneId()`, which is automatically inserted into the
logged message.

When spawining go routines, pass `l` around, or use one of the Derive functions when
a new correlation ID is needed.

Optionally, an "outer ID" can be assigned with `SetJourneyId()`. This function is useful
to correlate a transaction that involves many lanes, or to correlate with an externally
generated ID. The journey id is inherited by derived lanes.

For example, a front end might generate a journey ID, passing it with its REST
request to a go server that logs its activity via lanes. By setting the journey ID to
what the front end has generated, the lanes will be correlated with front end logging.

Another lane can "tee" from a source lane. For example, it might be desired to tee a
testing lane from a logging lane, and then a unit test can verify certain log messages
occur during the test.

## Utility Functions

### LogObject
`lane.LogObject` provides access to the common implementation of `InfoObject`, `InfoError`, etc., for implementing extended lane types.

### CaptureObject
`lane.CaptureObject` exposes the function that turns an object into one that can be
used with `json.Marshal` without losing private data. It does not retain `json`
type annotations however.

### DiffObject
`lane.DiffObject` returns a string that describes the differences between two objects,
or an empty string if the objects are identical.

For example, assume a and b are instances of a street address struct, and are
passed into a function like this:

```go
func compare(l lane.Lane, a, b any) {
	if !reflect.DeepEqual(a, b) {
		l.Infof("address change: %s", lane.DiffObject(a, b))
	}
}
```

Assume street addresses a and b are different. The log output will look something like this:

```
2024/07/11 13:20:26 INFO {e87ed2b1ed} address change: [Address: "2727 S Parker Rd" -> "407 Weston St"][City: "Dallas" -> "Kirby"][Zip: "75233" -> "78219"]
```

Only the changed fields are logged. Notice Texas is not shown in the change.

# Types of Lanes

- `NewLogLane` log messages go to the standard Go `log` infrastructure. Access the `log`
  instance via `Logger()` to set flags, add a prefix, or change output I/O.
- `NewDiskLane` like a "log lane" but writes output to a file.
- `NewTestingLane` captures log messages into a buffer and provides helpers for unit tests:

  - `VerifyEvents()`, `VerifyEventText()` - check for exact log messages
  - `FindEvents()`, `FindEventText()` - check logged messages for specific logging events
  - `EventsToString()` - stringify the logged messages for verification by the unit test

  A testing lane also has the API `WantDescendantEvents()` to enable (or disable) capture of
  derived testing lane activity. This is useful to verify a child task reaches an expected
  logging point.

- `NewNullLane` creates a lane that does not log but still has the context functionality.
  Logging is similar to `log.SetOutput(io.Discard)` - fatal errors still terminate the app.

Check out other projects, such as [go-lane-gin](https://github.com/jimsnab/go-lane-gin) or
[go-lane-opensearch](https://github.com/jimsnab/go-lane-opensearch) for additional lane types.

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

# Panic Handler

Fatal messages result in a panic. The panic handler can be replaced by test code to
verify a fatal condition is reached within a test.

An ordinary unrecovered panic won't allow other go routines to continue, because,
obviously, the process normally terminates on a panic. A test must ensure all go
routines started by the test are stopped by its replacement panic handler.

At minimum, the test's replacement panic handler must not let the panicking go
routine continue execution (it should call `runtime.Goexit()`).

# OptionalContext

`lane.OptionalContext` is an alias type for `context.Context`. It's used because linters want
to enforce callers to call `context.Background()` or `context.TODO()`. Callers can certainly
do that, but the linter rule is questionable -- just pass `nil`, what's the problem?

# Parent Context

Ordinary `context` does not provide a way to navigate to a parent. But sometimes this
is wanted, such as when implementing diagnostics.

The parent lane ID is available as a `context.Value()` under the name `ParentLaneIdKey`:

The parent lane (which is a `context`) is also available by calling `Lane.Parent()`.

```go
	lOne := lane.NewLogLane(nil)
	lTwo := lOne.Derive()
	fmt.Printf("lOne %s == lTwo's parent %v\n", lOne.LaneId(), lTwo.Value(lane.ParentLaneIdKey))
	
	lThree := lTwo.Parent()
	fmt.Printf("lOne %s == lThree %s\n", lOne.Lane(), lThree.LaneId())
```
