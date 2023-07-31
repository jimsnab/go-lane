# go-lane

A "lane" is a context that has logging associated. It is a melding of Go's `log` and its `context`.

# Basic Use

```
import (
    "context"
    "github.com/jimsnab/go-lane"
)

func myFunc() {
    l := NewLogLane(context.Background())

    l.Info("log something")
}
```

At the root, a lane needs a context, and that is typically `context.Background()`. From there, instead of
passing a `context` instance as the first parameter, pass the lane `l`.

```
func someFunc(l lane.Lane) {
     // use l like a context instance, or call one of its interface members
}
```

# Interface
```
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

		Derive() Lane
		DeriveWithCancel() (Lane, context.CancelFunc)
		DeriveWithDeadline(deadline time.Time) (Lane, context.CancelFunc)
		DeriveWithTimeout(duration time.Duration) (Lane, context.CancelFunc)
	}
```

For the most part, application code will use the logging functions (Trace, Debug, ...).

A correlation ID is provided via `LaneId()`, which is automatically inserted into the
logged message.

When spawining go routines, pass `l` around, or use one of the Derive functions when
a new correlation ID is needed.

# Types of Lanes
* `NewLogLane` log messages go to the standard Go `log()` infrastructure.
* `NewTestingLane` captures log messages into a buffer and provides `VerifyEvents()`,
  `VerifyEventText()` and `EventsToString()` for use in unit test code that checks the log to confirm
  an expected result.
* `NewNullLane` creates a lane that does not log but still has the context functionality.

Normally the production code uses a log lane, and unit tests use a testing lane; a null
lane is handy in unit tests to disable logging out of scope of the test.

The code doing the logging or using the context should not care what kind of lane it
is given to use.
