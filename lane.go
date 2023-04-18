package lane

import (
	"context"
	"log"
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
	}
)
