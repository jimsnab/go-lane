package lane

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// NewSlogHandler returns a structured logging handler that routes records through l.
// Records retain standard slog attributes and groups, then use the lane's configured
// log level, filters, tees, and output destination.
func NewSlogHandler(l Lane) slog.Handler {
	writer := &slogLaneWriter{lane: l}
	return &slogLaneHandler{
		lane:   l,
		writer: writer,
	}
}

type slogLaneHandler struct {
	lane    Lane
	writer  *slogLaneWriter
	changes []slogHandlerChange
}

type slogHandlerChange struct {
	attrs []slog.Attr
	group string
}

type slogLaneWriter struct {
	mu    sync.Mutex
	lane  Lane
	level LaneLogLevel
}

func (h *slogLaneHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.lane.IsLevelEnabled(laneLogLevel(level))
}

func (h *slogLaneHandler) Handle(ctx context.Context, record slog.Record) error {
	h.writer.mu.Lock()
	defer h.writer.mu.Unlock()

	h.writer.level = laneLogLevel(record.Level)
	var handler slog.Handler = slog.NewTextHandler(h.writer, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: removeLaneDuplicateSlogAttrs,
	})
	for _, change := range h.changes {
		if change.attrs != nil {
			handler = handler.WithAttrs(change.attrs)
		} else {
			handler = handler.WithGroup(change.group)
		}
	}
	return handler.Handle(ctx, record)
}

func (h *slogLaneHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &slogLaneHandler{
		lane:    h.lane,
		writer:  h.writer,
		changes: appendSlogHandlerChange(h.changes, slogHandlerChange{attrs: attrs}),
	}
}

func (h *slogLaneHandler) WithGroup(name string) slog.Handler {
	return &slogLaneHandler{
		lane:    h.lane,
		writer:  h.writer,
		changes: appendSlogHandlerChange(h.changes, slogHandlerChange{group: name}),
	}
}

func appendSlogHandlerChange(changes []slogHandlerChange, change slogHandlerChange) []slogHandlerChange {
	cloned := make([]slogHandlerChange, len(changes), len(changes)+1)
	copy(cloned, changes)
	return append(cloned, change)
}

func (w *slogLaneWriter) Write(message []byte) (int, error) {
	w.log(strings.TrimSuffix(string(message), "\n"))
	return len(message), nil
}

func (w *slogLaneWriter) log(message string) {
	switch w.level {
	case LogLevelTrace:
		w.lane.Trace(message)
	case LogLevelDebug:
		w.lane.Debug(message)
	case LogLevelInfo:
		w.lane.Info(message)
	case LogLevelWarn:
		w.lane.Warn(message)
	default:
		w.lane.Error(message)
	}
}

func laneLogLevel(level slog.Level) LaneLogLevel {
	switch {
	case level < slog.LevelDebug:
		return LogLevelTrace
	case level < slog.LevelInfo:
		return LogLevelDebug
	case level < slog.LevelWarn:
		return LogLevelInfo
	case level < slog.LevelError:
		return LogLevelWarn
	default:
		return LogLevelError
	}
}

func removeLaneDuplicateSlogAttrs(_ []string, attr slog.Attr) slog.Attr {
	switch attr.Key {
	case slog.TimeKey, slog.LevelKey:
		return slog.Attr{}
	default:
		return attr
	}
}