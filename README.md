# go-lane

A "lane" is a context that integrates logging, combining Go's `log` package with its `context`.

A lane is designed to be passed into all functions, serving as a regular `context` for tasks like cancellation,
while also replacing `log` for logging purposes.

The logging interface offers various tools for detailed diagnostics, including correlation IDs, stack traces,
easy logging of structs (even private fields), log capturing for test verification, recording structural
differences, and more.

Related projects extend this unified logging approach, such as
[go-lane-gin](https://github.com/jimsnab/go-lane-gin) and
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

	LogStack(message string)
	LogStackTrim(message string, skippedCallers int)

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

For the most part, application code will use the logging functions (`Trace`, `Debug`, etc.).

Each log level offers:

* a `Sprint` version (e.g., `Info` or `Error`)
* a `Sprintf` version (e.g., `Infof` or `Errorf`)
* an object logger (e.g., `InfoObject` or `ErrorObject`)

The object logger converts an object to JSON, including private fields.

A correlation ID is provided via `LaneId()`, which is automatically included in logged messages.

When spawning goroutines, pass `l` (the lane) around. Use one of the `Derive` functions if a new
correlation ID is needed.

Optionally, an "outer ID" can be assigned with `SetJourneyId()`. This function is useful for
correlating transactions that involve multiple lanes or for linking with an externally generated ID.
The journey ID is inherited by derived lanes.

For example, a front-end application might generate a journey ID and pass it with its REST request
to a Go server that logs activity via lanes. By setting the journey ID to match what the front end
generated, the lanes will be correlated with front-end logging.

Another lane can "tee" from a source lane. For instance, you might tee a testing lane from a logging
lane, allowing a unit test to verify that certain log messages are generated during the test.

When a log message is sent to a tee, the receiving lane will log the journey and lane IDs using the
originating IDs, and not the receiving lane's IDs.

## Utility Functions

### LogObject
`lane.LogObject` provides access to the common implementation of `InfoObject`, `ErrorObject`, etc., for implementing extended lane types.

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
  - `Contains()` - checks if text is found in any captured log message

  A testing lane also has the API `WantDescendantEvents()` to enable (or disable) capture of
  derived testing lane activity. This is useful to verify a child task reaches an expected
  logging point.

- `NewNullLane` creates a lane that does not log but still has the context functionality.
  Logging is similar to `log.SetOutput(io.Discard)` - fatal errors still terminate the app.

Check out other projects, such as [go-lane-gin](https://github.com/jimsnab/go-lane-gin) or
[go-lane-opensearch](https://github.com/jimsnab/go-lane-opensearch) for additional lane types.

# Stack Trace

Stacks can be logged using `LogStack()`, or `LogStackTrim()` to remove some of the callers
from the top of the stack.

Stack trace logging can be enabled on a per level basis. For example, to enable stack
trace output for `ERROR`:

```go
func example() {
	l := NewLogLane(context.Background())
	l.EnableStackTrace(lane.LogLevelError, true)
	l.Error("sample")   // stack trace is logged also
}
```

The test lane includes a special option, `EnableSingleLineStackTrace()`, which logs the entire stack
trace as a single test event. This creates a more predictable test event list compared to traditional
stack traces, where each caller is logged as a separate event.

# Max Message Length
The length of a single log message can be length-constrained. Call `SetLengthConstraint()` to
do that.

# Panic Handler

Fatal messages trigger a panic. In test code, the panic handler can be replaced to verify that a
fatal condition is reached during the test.

An ordinary unrecovered panic will prevent other goroutines from continuing, as the process
typically terminates on a panic. A test must ensure that all goroutines started by the test are
stopped by the replacement panic handler.

At a minimum, the test's replacement panic handler must prevent the panicking goroutine from
continuing execution (it should call `runtime.Goexit()`).

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
	fmt.Printf("lOne %s == lThree %s\n", lOne.LaneId(), lThree.LaneId())
```
