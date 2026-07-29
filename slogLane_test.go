package lane

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestSlogLoggerRoutesRecordsThroughLane(t *testing.T) {
	lane := NewTestingLane(context.Background())
	lane.SetJourneyId("journey")
	lane.SetMetadata("tenant", "acme")

	logger := lane.SlogLogger().With("request", 42).WithGroup("http")
	logger.LogAttrs(context.Background(), slog.LevelInfo, "request complete", slog.Int("status", 200))

	events := lane.EventsToString()
	for _, expected := range []string{
		"INFO\t",
		"msg=\"request complete\"",
		"lane_id=" + lane.LaneId(),
		"journey_id=journey",
		"tenant=acme",
		"request=42",
		"http.status=200",
	} {
		if !strings.Contains(events, expected) {
			t.Errorf("expected %q in %s", expected, events)
		}
	}
}

func TestSlogHandlerMapsLevelsAndRespectsLaneFiltering(t *testing.T) {
	lane := NewTestingLane(context.Background())
	lane.SetLogLevel(LogLevelInfo)
	logger := lane.SlogLogger()

	if logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should be disabled at info level")
	}
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info should be enabled at info level")
	}

	logger.Log(context.Background(), slog.LevelDebug-1, "trace")
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	lines := strings.Split(lane.EventsToString(), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three enabled slog events, got %d: %s", len(lines), lane.EventsToString())
	}
	for index, expected := range []string{"INFO\t", "WARN\t", "ERROR\t"} {
		if !strings.HasPrefix(lines[index], expected) {
			t.Errorf("event %d has text %q, want prefix %q", index, lines[index], expected)
		}
	}
}

func TestLaneLogLevelMapping(t *testing.T) {
	for _, test := range []struct {
		level slog.Level
		want  LaneLogLevel
	}{
		{level: slog.LevelDebug - 1, want: LogLevelTrace},
		{level: slog.LevelDebug, want: LogLevelDebug},
		{level: slog.LevelInfo, want: LogLevelInfo},
		{level: slog.LevelWarn, want: LogLevelWarn},
		{level: slog.LevelError, want: LogLevelError},
		{level: slog.LevelError + 1, want: LogLevelError},
	} {
		if got := laneLogLevel(test.level); got != test.want {
			t.Errorf("laneLogLevel(%d) = %d, want %d", test.level, got, test.want)
		}
	}
}

func TestSlogLoggerRetainsGroupsAndLogValuer(t *testing.T) {
	lane := NewTestingLane(context.Background())
	logger := lane.SlogLogger().WithGroup("service").With("name", "api")

	logger.Info("ready", slog.Group("request", slog.String("method", "GET")))

	events := lane.EventsToString()
	for _, expected := range []string{"service.name=api", "service.request.method=GET"} {
		if !strings.Contains(events, expected) {
			t.Errorf("expected %q in %s", expected, events)
		}
	}
}

func TestSlogLoggerUsesFilterLane(t *testing.T) {
	captured := NewTestingLane(context.Background())
	filtered := NewRegexFilterLane(captured, "audit")
	logger := filtered.SlogLogger()

	logger.Info("audit event")
	logger.Info("ordinary event")

	if !captured.Contains("audit event") || captured.Contains("ordinary event") {
		t.Errorf("filter did not receive expected slog events: %s", captured.EventsToString())
	}
}