package lane

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Test NewRegexFilterLane constructor
func TestAddFilterTee(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Add audit filter tee
	auditTee := NewTestingLane(context.Background())
	AddFilterTee(tl, auditTee, `^\[AUDIT\]`)

	// Add unfiltered tee
	allTee := NewTestingLane(context.Background())
	AddFilterTee(tl, allTee, "")

	// Log some messages
	tl.Info("[AUDIT] user login: alice")
	tl.Debug("debug message")
	tl.Warn("[AUDIT] failed login: bob")

	// Main lane should have all messages
	if !tl.VerifyEvents([]*LaneEvent{
		{Level: "INFO", Message: "[AUDIT] user login: alice"},
		{Level: "DEBUG", Message: "debug message"},
		{Level: "WARN", Message: "[AUDIT] failed login: bob"},
	}) {
		t.Error("main lane missing events")
	}

	// Audit tee should only have [AUDIT] messages
	if !auditTee.VerifyEvents([]*LaneEvent{
		{Level: "INFO", Message: "[AUDIT] user login: alice"},
		{Level: "WARN", Message: "[AUDIT] failed login: bob"},
	}) {
		t.Error("audit tee has wrong events")
	}

	// All tee should have all messages
	if !allTee.VerifyEvents([]*LaneEvent{
		{Level: "INFO", Message: "[AUDIT] user login: alice"},
		{Level: "DEBUG", Message: "debug message"},
		{Level: "WARN", Message: "[AUDIT] failed login: bob"},
	}) {
		t.Error("all tee missing events")
	}
}

func TestNewRegexFilterLane(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Simple one-liner
	fl := NewRegexFilterLane(tl, `^\[AUDIT\]`)

	fl.Info("[AUDIT] user login")
	fl.Info("regular message")
	fl.Error("[AUDIT] security issue")

	if !tl.Contains("[AUDIT] user login") {
		t.Error("Should contain audit login")
	}
	if !tl.Contains("[AUDIT] security issue") {
		t.Error("Should contain audit security")
	}
	if tl.Contains("regular message") {
		t.Error("Should not contain regular message")
	}
}

// Test NewLevelFilterLane constructor
func TestNewLevelFilterLane(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Simple one-liner
	fl := NewLevelFilterLane(tl, LogLevelWarn)

	fl.Info("info message")
	fl.Warn("warning message")
	fl.Error("error message")

	if !tl.Contains("warning message") {
		t.Error("Should contain warning")
	}
	if !tl.Contains("error message") {
		t.Error("Should contain error")
	}
	if tl.Contains("info message") {
		t.Error("Should not contain info")
	}
}

// Test NewRegexFilter helper
func TestNewRegexFilter(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Filter for messages starting with [AUDIT]
	filter := NewRegexFilter(`^\[AUDIT\]`)
	fl := NewFilterLane(tl, filter)

	fl.Info("[AUDIT] user login")
	fl.Info("regular message")
	fl.Error("[AUDIT] security issue")
	fl.Debug("debug info")

	if !tl.Contains("[AUDIT] user login") {
		t.Error("Should contain audit login message")
	}
	if !tl.Contains("[AUDIT] security issue") {
		t.Error("Should contain audit security message")
	}
	if tl.Contains("regular message") {
		t.Error("Should not contain non-audit message")
	}
	if tl.Contains("debug info") {
		t.Error("Should not contain debug message")
	}
}

// Test NewRegexFilter with case-insensitive pattern
func TestNewRegexFilterCaseInsensitive(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Filter for messages containing "error" or "warning" (case-insensitive)
	filter := NewRegexFilter(`(?i)(error|warning)`)
	fl := NewFilterLane(tl, filter)

	fl.Info("ERROR detected")
	fl.Info("Warning: low memory")
	fl.Info("regular message")
	fl.Error("critical error")

	if !tl.Contains("ERROR detected") {
		t.Error("Should match ERROR")
	}
	if !tl.Contains("Warning: low memory") {
		t.Error("Should match Warning")
	}
	if !tl.Contains("critical error") {
		t.Error("Should match error")
	}
	if tl.Contains("regular message") {
		t.Error("Should not match regular message")
	}
}

// Test NewLevelFilter helper
func TestNewLevelFilter(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Only log warnings and above
	filter := NewLevelFilter(LogLevelWarn)
	fl := NewFilterLane(tl, filter)

	fl.Trace("trace message")
	fl.Debug("debug message")
	fl.Info("info message")
	fl.Warn("warning message")
	fl.Error("error message")

	if !tl.Contains("warning message") {
		t.Error("Should contain warning")
	}
	if !tl.Contains("error message") {
		t.Error("Should contain error")
	}
	if tl.Contains("trace message") || tl.Contains("debug message") || tl.Contains("info message") {
		t.Error("Should not contain trace/debug/info messages")
	}
}

// Test combining regex and level filters
func TestCombinedFilters(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Combine: must be ERROR or WARN level AND contain [CRITICAL]
	combinedFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		levelFilter := NewLevelFilter(LogLevelWarn)
		regexFilter := NewRegexFilter(`\[CRITICAL\]`)
		return levelFilter(lane, level, msg) && regexFilter(lane, level, msg)
	}

	fl := NewFilterLane(tl, combinedFilter)

	fl.Info("[CRITICAL] info message")    // Wrong level
	fl.Warn("[CRITICAL] warning message") // Match
	fl.Warn("regular warning")            // Wrong pattern
	fl.Error("[CRITICAL] error message")  // Match
	fl.Error("regular error")             // Wrong pattern

	if !tl.Contains("[CRITICAL] warning message") {
		t.Error("Should contain critical warning")
	}
	if !tl.Contains("[CRITICAL] error message") {
		t.Error("Should contain critical error")
	}
	if tl.Contains("[CRITICAL] info message") || tl.Contains("regular warning") || tl.Contains("regular error") {
		t.Error("Should not contain non-matching messages")
	}
}

// Test basic filtering by message prefix
func TestFilterLanePrefix(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Create a filter that only passes messages with [AUDIT] prefix
	auditFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.HasPrefix(msg, "[AUDIT]")
	}

	fl := NewFilterLane(tl, auditFilter)

	// Log various messages
	fl.Info("[AUDIT] user logged in")
	fl.Info("debug information")
	fl.Info("[AUDIT] file accessed")
	fl.Debug("more debug info")
	fl.Error("[AUDIT] security violation")
	fl.Error("regular error")

	// Verify only audit messages were logged
	events := []*LaneEvent{
		{Level: "INFO", Message: "[AUDIT] user logged in"},
		{Level: "INFO", Message: "[AUDIT] file accessed"},
		{Level: "ERROR", Message: "[AUDIT] security violation"},
	}

	if !tl.VerifyEvents(events) {
		t.Errorf("Filter did not work correctly. Got:\n%s", tl.EventsToString())
	}
}

// Test filtering by log level
func TestFilterLaneByLevel(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Create a filter that only passes error and fatal messages
	errorOnlyFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return level >= LogLevelError
	}

	fl := NewFilterLane(tl, errorOnlyFilter)

	// Log various levels
	fl.Trace("trace message")
	fl.Debug("debug message")
	fl.Info("info message")
	fl.Warn("warn message")
	fl.Error("error message")
	fl.PreFatal("prefatal message")

	// Verify only error and prefatal were logged
	events := []*LaneEvent{
		{Level: "ERROR", Message: "error message"},
		{Level: "FATAL", Message: "prefatal message"},
	}

	if !tl.VerifyEvents(events) {
		t.Errorf("Level filter did not work correctly. Got:\n%s", tl.EventsToString())
	}
}

// Test that nil filter passes everything through
func TestFilterLaneNilFilter(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	fl.Info("test1")
	fl.Debug("test2")
	fl.Error("test3")

	events := []*LaneEvent{
		{Level: "INFO", Message: "test1"},
		{Level: "DEBUG", Message: "test2"},
		{Level: "ERROR", Message: "test3"},
	}

	if !tl.VerifyEvents(events) {
		t.Errorf("Nil filter should pass everything through")
	}
}

// Test formatted logging methods
func TestFilterLaneFormatted(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.Contains(msg, "keep")
	}

	fl := NewFilterLane(tl, filter)

	fl.Infof("keep this %d", 1)
	fl.Infof("drop this %d", 2)
	fl.Debugf("keep this too %s", "test")
	fl.Errorf("drop error %d", 3)

	events := []*LaneEvent{
		{Level: "INFO", Message: "keep this 1"},
		{Level: "DEBUG", Message: "keep this too test"},
	}

	if !tl.VerifyEvents(events) {
		t.Errorf("Formatted filter did not work correctly. Got:\n%s", tl.EventsToString())
	}
}

// Test object logging
func TestFilterLaneObject(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.HasPrefix(msg, "KEEP")
	}

	fl := NewFilterLane(tl, filter)

	type testStruct struct {
		Field string
	}

	fl.InfoObject("KEEP: data", testStruct{Field: "value1"})
	fl.InfoObject("DROP: data", testStruct{Field: "value2"})

	// Just check for the message - EventsToString shows "INFO    KEEP: data: ..."
	if !tl.Contains("KEEP: data") {
		t.Errorf("Object filter did not work correctly. Got:\n%s", tl.EventsToString())
	}

	if tl.Contains("DROP") {
		t.Error("Filtered message should not appear")
	}
}

// Test that context methods work correctly
func TestFilterLaneContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "testKey", "testValue")
	tl := NewTestingLane(ctx)
	fl := NewFilterLane(tl, nil)

	if fl.Value("testKey") != "testValue" {
		t.Error("Context value not passed through")
	}

	if fl.LaneId() != tl.LaneId() {
		t.Error("Lane ID not passed through")
	}
}

// Test metadata passthrough
func TestFilterLaneMetadata(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	fl.SetMetadata("key1", "value1")
	if fl.GetMetadata("key1") != "value1" {
		t.Error("Metadata not stored correctly")
	}

	if tl.GetMetadata("key1") != "value1" {
		t.Error("Metadata not visible in wrapped lane")
	}
}

// Test journey ID
func TestFilterLaneJourneyId(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	fl.SetJourneyId("journey123")
	if fl.JourneyId() != "journey123" {
		t.Error("Journey ID not set correctly")
	}

	if tl.JourneyId() != "journey123" {
		t.Error("Journey ID not visible in wrapped lane")
	}
}

// Test log level configuration
func TestFilterLaneLogLevel(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	prior := fl.SetLogLevel(LogLevelWarn)
	if prior != LogLevelTrace {
		t.Errorf("Expected prior level to be Trace, got %d", prior)
	}

	// Even though filter passes everything, log level should filter
	fl.Trace("should not appear")
	fl.Debug("should not appear")
	fl.Info("should not appear")
	fl.Warn("should appear")
	fl.Error("should appear")

	events := []*LaneEvent{
		{Level: "WARN", Message: "should appear"},
		{Level: "ERROR", Message: "should appear"},
	}

	if !tl.VerifyEvents(events) {
		t.Errorf("Log level filtering not working. Got:\n%s", tl.EventsToString())
	}
}

// Test derivation maintains filter
func TestFilterLaneDerive(t *testing.T) {
	tl := NewTestingLane(context.Background())
	tl.WantDescendantEvents(true) // Capture events from derived lanes

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.HasPrefix(msg, "[KEEP]")
	}

	fl := NewFilterLane(tl, filter)
	fl2 := fl.Derive()

	fl.Info("[KEEP] message 1")
	fl2.Info("[KEEP] message 2")
	fl2.Info("[DROP] message 3")

	if !tl.FindEventText("INFO\t[KEEP] message 1") {
		t.Error("Parent filter not working")
	}

	if !tl.FindEventText("INFO\t[KEEP] message 2") {
		t.Errorf("Derived filter not working. Got:\n%s", tl.EventsToString())
	}

	if tl.Contains("[DROP]") {
		t.Error("Derived lane should maintain filter")
	}
}

// Test derivation with cancel
func TestFilterLaneDeriveWithCancel(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return true
	}

	fl := NewFilterLane(tl, filter)
	fl2, cancel := fl.DeriveWithCancel()

	fl2.Info("test message")

	if fl2.Err() != nil {
		t.Error("Context should not be cancelled yet")
	}

	cancel()

	if fl2.Err() == nil {
		t.Error("Context should be cancelled")
	}
}

// Test tee with filter - audit log use case
func TestFilterLaneTeeAuditUseCase(t *testing.T) {
	// Create two testing lanes - one for audit, one for debug
	auditLog := NewTestingLane(context.Background())
	debugLog := NewTestingLane(context.Background())

	// Create filters
	auditFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.HasPrefix(msg, "[AUDIT]")
	}

	debugFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return true // accepts everything
	}

	// Wrap the lanes with filters
	filteredAudit := NewFilterLane(auditLog, auditFilter)
	filteredDebug := NewFilterLane(debugLog, debugFilter)

	// Create main lane and add tees
	mainLane := NewTestingLane(context.Background())
	mainLane.AddTee(filteredAudit)
	mainLane.AddTee(filteredDebug)

	// Log various messages
	mainLane.Info("[AUDIT] user login")
	mainLane.Info("regular debug info")
	mainLane.Error("[AUDIT] security violation")
	mainLane.Error("regular error")

	// Verify audit log only has audit messages
	auditEvents := []*LaneEvent{
		{Level: "INFO", Message: "[AUDIT] user login"},
		{Level: "ERROR", Message: "[AUDIT] security violation"},
	}

	if !auditLog.VerifyEvents(auditEvents) {
		t.Errorf("Audit log should only have audit messages. Got:\n%s", auditLog.EventsToString())
	}

	// Verify debug log has all messages
	debugEvents := []*LaneEvent{
		{Level: "INFO", Message: "[AUDIT] user login"},
		{Level: "INFO", Message: "regular debug info"},
		{Level: "ERROR", Message: "[AUDIT] security violation"},
		{Level: "ERROR", Message: "regular error"},
	}

	if !debugLog.VerifyEvents(debugEvents) {
		t.Errorf("Debug log should have all messages. Got:\n%s", debugLog.EventsToString())
	}
}

// Test stack trace filtering
func TestFilterLaneStackTrace(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.Contains(msg, "keep")
	}

	fl := NewFilterLane(tl, filter)

	fl.LogStack("keep stack")
	fl.LogStack("drop stack")

	if !tl.Contains("keep stack") {
		t.Error("Stack trace should be kept")
	}

	if tl.Contains("drop stack") {
		t.Error("Stack trace should be dropped")
	}
}

// Test complex filter combining level and content
func TestFilterLaneComplex(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Filter: errors with [CRITICAL] prefix OR any warning
	complexFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		if level == LogLevelError && strings.HasPrefix(msg, "[CRITICAL]") {
			return true
		}
		if level == LogLevelWarn {
			return true
		}
		return false
	}

	fl := NewFilterLane(tl, complexFilter)

	fl.Info("info message")
	fl.Warn("warning message")
	fl.Error("regular error")
	fl.Error("[CRITICAL] critical error")
	fl.Debug("debug message")

	events := []*LaneEvent{
		{Level: "WARN", Message: "warning message"},
		{Level: "ERROR", Message: "[CRITICAL] critical error"},
	}

	if !tl.VerifyEvents(events) {
		t.Errorf("Complex filter did not work correctly. Got:\n%s", tl.EventsToString())
	}
}

// Test that AddTee and RemoveTee work correctly
func TestFilterLaneTeeManagement(t *testing.T) {
	tl1 := NewTestingLane(context.Background())
	tl2 := NewTestingLane(context.Background())
	fl := NewFilterLane(tl1, nil)

	fl.AddTee(tl2)

	tees := fl.Tees()
	if len(tees) != 1 {
		t.Errorf("Expected 1 tee, got %d", len(tees))
	}

	fl.Info("test message")

	if !tl2.FindEventText("INFO\ttest message") {
		t.Error("Tee should receive messages")
	}

	fl.RemoveTee(tl2)

	tees = fl.Tees()
	if len(tees) != 0 {
		t.Errorf("Expected 0 tees after removal, got %d", len(tees))
	}
}

// Test parent access through filter
func TestFilterLaneParent(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	// Initial lane has no parent
	if fl.Parent() != nil {
		t.Error("Initial lane should have no parent")
	}

	// Derive a child and check parent
	child := fl.Derive()
	parent := child.Parent()

	if parent == nil {
		t.Error("Derived lane should have a parent")
	}

	// Parent should also be a filter lane
	if _, ok := parent.(*filterLane); !ok {
		t.Error("Parent should also be wrapped in filter")
	}
}

// Test length constraint passthrough
func TestFilterLaneLengthConstraint(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	prior := fl.SetLengthConstraint(10)
	if prior != 0 {
		t.Errorf("Expected prior constraint to be 0, got %d", prior)
	}

	fl.Info("this is a very long message that should be truncated")

	// The message should be constrained in the wrapped lane
	if !tl.Contains("…") {
		t.Error("Message should be truncated")
	}
}

// Test Close() passthrough
func TestFilterLaneClose(t *testing.T) {
	// Create a null lane (which has a Close method)
	nl := NewNullLane(context.Background())
	fl := NewFilterLane(nl, nil)

	// Should not panic
	fl.Close()
}

// Test all derive variations work correctly
func TestFilterLaneAllDeriveVariations(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	// DeriveWithoutCancel
	d1 := fl.DeriveWithoutCancel()
	if d1 == nil {
		t.Error("DeriveWithoutCancel failed")
	}

	// DeriveWithDeadline
	d2, c2 := fl.DeriveWithDeadline(time.Now().Add(time.Hour))
	if d2 == nil {
		t.Error("DeriveWithDeadline failed")
	}
	c2()

	// DeriveWithTimeout
	d3, c3 := fl.DeriveWithTimeout(time.Hour)
	if d3 == nil {
		t.Error("DeriveWithTimeout failed")
	}
	c3()

	// DeriveReplaceContext
	newCtx := context.WithValue(context.Background(), "new", "value")
	d4 := fl.DeriveReplaceContext(newCtx)
	if d4 == nil {
		t.Error("DeriveReplaceContext failed")
	}
	if d4.Value("new") != "value" {
		t.Error("New context not applied")
	}
}

// Test *Cause derive variations
func TestFilterLaneCauseDeriveVariations(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	// DeriveWithCancelCause
	d1, c1 := fl.DeriveWithCancelCause()
	if d1 == nil {
		t.Error("DeriveWithCancelCause failed")
	}
	c1(nil)

	// DeriveWithDeadlineCause
	d2, c2 := fl.DeriveWithDeadlineCause(time.Now().Add(time.Hour), context.DeadlineExceeded)
	if d2 == nil {
		t.Error("DeriveWithDeadlineCause failed")
	}
	c2()

	// DeriveWithTimeoutCause
	d3, c3 := fl.DeriveWithTimeoutCause(time.Hour, context.DeadlineExceeded)
	if d3 == nil {
		t.Error("DeriveWithTimeoutCause failed")
	}
	c3()
}

// Test all formatted logging methods
func TestFilterLaneAllFormattedMethods(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.Contains(msg, "keep")
	}

	fl := NewFilterLane(tl, filter)

	fl.Tracef("keep %d", 1)
	fl.Tracef("drop %d", 2)
	fl.Warnf("keep %d", 3)
	fl.Warnf("drop %d", 4)
	fl.PreFatalf("keep %d", 5)
	fl.PreFatalf("drop %d", 6)

	if !tl.Contains("keep 1") {
		t.Error("Tracef filter failed")
	}
	if tl.Contains("drop 2") {
		t.Error("Tracef should filter")
	}
	if !tl.Contains("keep 3") {
		t.Error("Warnf filter failed")
	}
	if tl.Contains("drop 4") {
		t.Error("Warnf should filter")
	}
	if !tl.Contains("keep 5") {
		t.Error("PreFatalf filter failed")
	}
	if tl.Contains("drop 6") {
		t.Error("PreFatalf should filter")
	}
}

// Test all Object logging methods
func TestFilterLaneAllObjectMethods(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.HasPrefix(msg, "KEEP")
	}

	fl := NewFilterLane(tl, filter)

	type testData struct {
		Value string
	}

	fl.TraceObject("KEEP trace", testData{"t1"})
	fl.TraceObject("DROP trace", testData{"t2"})
	fl.DebugObject("KEEP debug", testData{"d1"})
	fl.DebugObject("DROP debug", testData{"d2"})
	fl.WarnObject("KEEP warn", testData{"w1"})
	fl.WarnObject("DROP warn", testData{"w2"})
	fl.ErrorObject("KEEP error", testData{"e1"})
	fl.ErrorObject("DROP error", testData{"e2"})
	fl.PreFatalObject("KEEP prefatal", testData{"pf1"})
	fl.PreFatalObject("DROP prefatal", testData{"pf2"})

	if !tl.Contains("KEEP trace") {
		t.Error("TraceObject filter failed")
	}
	if !tl.Contains("KEEP debug") {
		t.Error("DebugObject filter failed")
	}
	if !tl.Contains("KEEP warn") {
		t.Error("WarnObject filter failed")
	}
	if !tl.Contains("KEEP error") {
		t.Error("ErrorObject filter failed")
	}
	if !tl.Contains("KEEP prefatal") {
		t.Error("PreFatalObject filter failed")
	}

	if tl.Contains("DROP trace") || tl.Contains("DROP debug") || tl.Contains("DROP warn") ||
		tl.Contains("DROP error") || tl.Contains("DROP prefatal") {
		t.Error("Filtered object messages should not appear")
	}
}

// Test Fatal methods with panic handler
func TestFilterLaneFatalMethods(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.Contains(msg, "keep")
	}

	fl := NewFilterLane(tl, filter)

	panicCalled := false
	fl.SetPanicHandler(func() {
		panicCalled = true
	})

	fl.Fatal("keep fatal")
	if !panicCalled {
		t.Error("Fatal should trigger panic")
	}
	if !tl.Contains("keep fatal") {
		t.Error("Fatal message not logged")
	}

	panicCalled = false
	fl.Fatalf("keep fatal %d", 123)
	if !panicCalled {
		t.Error("Fatalf should trigger panic")
	}
	if !tl.Contains("keep fatal 123") {
		t.Error("Fatalf message not logged")
	}

	type testData struct {
		Field string
	}
	panicCalled = false
	fl.FatalObject("keep object", testData{"value"})
	if !panicCalled {
		t.Error("FatalObject should trigger panic")
	}
	if !tl.Contains("keep object") {
		t.Error("FatalObject message not logged")
	}
}

// Test Fatal methods with filtering
func TestFilterLaneFatalFiltering(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.Contains(msg, "keep")
	}

	fl := NewFilterLane(tl, filter)

	panicCalled := false
	fl.SetPanicHandler(func() {
		panicCalled = true
	})

	fl.Fatal("drop this")
	if panicCalled {
		t.Error("Fatal should not trigger panic when filtered")
	}
	if tl.Contains("drop this") {
		t.Error("Filtered fatal should not appear")
	}
}

// Test context Deadline and Done methods
func TestFilterLaneContextMethods(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	tl := NewTestingLane(ctx)
	fl := NewFilterLane(tl, nil)

	deadline, ok := fl.Deadline()
	if !ok {
		t.Error("Deadline should be set")
	}
	if deadline.IsZero() {
		t.Error("Deadline should not be zero")
	}

	select {
	case <-fl.Done():
		t.Error("Context should not be done yet")
	default:
		// Expected
	}

	cancel()

	<-fl.Done()
	if fl.Err() == nil {
		t.Error("Err should be set after cancel")
	}
}

// Test LogStackTrim
func TestFilterLaneLogStackTrim(t *testing.T) {
	tl := NewTestingLane(context.Background())

	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.Contains(msg, "keep")
	}

	fl := NewFilterLane(tl, filter)

	fl.LogStackTrim("keep stack", 1)
	if !tl.Contains("keep stack") {
		t.Error("LogStackTrim should log when filter passes")
	}

	fl.LogStackTrim("drop stack", 1)
	if tl.Contains("drop stack") {
		t.Error("LogStackTrim should filter")
	}
}

// Test Logger passthrough
func TestFilterLaneLogger(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	logger := fl.Logger()
	if logger == nil {
		t.Error("Logger should not be nil")
	}
}

// Test EnableStackTrace passthrough
func TestFilterLaneEnableStackTrace(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	wasEnabled := fl.EnableStackTrace(LogLevelError, true)
	if wasEnabled {
		t.Error("Stack trace should not be enabled initially for Error level")
	}

	wasEnabled = fl.EnableStackTrace(LogLevelError, false)
	if !wasEnabled {
		t.Error("Stack trace should now be enabled")
	}
}

// Test SetPanicHandler passthrough
func TestFilterLaneSetPanicHandler(t *testing.T) {
	tl := NewTestingLane(context.Background())
	fl := NewFilterLane(tl, nil)

	handlerCalled := false
	fl.SetPanicHandler(func() {
		handlerCalled = true
	})

	// Trigger a fatal to test panic handler
	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return true
	}
	fl2 := NewFilterLane(tl, filter)
	fl2.SetPanicHandler(func() {
		handlerCalled = true
	})

	fl2.Fatal("test")
	if !handlerCalled {
		t.Error("Panic handler should be called")
	}
}

// Test internal methods through comprehensive tee usage
func TestFilterLaneInternalMethodsViaTee(t *testing.T) {
	mainLane := NewTestingLane(context.Background())
	tee1 := NewTestingLane(context.Background())
	tee2 := NewTestingLane(context.Background())

	// Filter tee1 to only keep messages with "alpha"
	filter1 := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.Contains(msg, "alpha")
	}
	filteredTee1 := NewFilterLane(tee1, filter1)

	// Filter tee2 to keep all messages
	filteredTee2 := NewFilterLane(tee2, nil)

	mainLane.AddTee(filteredTee1)
	mainLane.AddTee(filteredTee2)

	// Test all internal methods through the tee mechanism
	mainLane.Trace("alpha trace")
	mainLane.Tracef("alpha tracef %d", 1)
	mainLane.Debug("beta debug")
	mainLane.Debugf("alpha debugf %d", 2)
	mainLane.Info("alpha info")
	mainLane.Infof("beta infof %d", 3)
	mainLane.Warn("alpha warn")
	mainLane.Warnf("beta warnf %d", 4)
	mainLane.Error("alpha error")
	mainLane.Errorf("beta errorf %d", 5)
	mainLane.PreFatal("alpha prefatal")
	mainLane.PreFatalf("beta prefatalf %d", 6)

	panicCalled := false
	mainLane.SetPanicHandler(func() {
		panicCalled = true
	})
	mainLane.Fatal("alpha fatal")
	mainLane.Fatalf("beta fatalf %d", 7)

	mainLane.LogStackTrim("alpha stack", 0)

	// Verify tee1 only got alpha messages (using Contains)
	if !tee1.Contains("alpha trace") {
		t.Error("tee1 should have alpha trace")
	}
	if !tee1.Contains("alpha debugf") {
		t.Error("tee1 should have alpha debugf")
	}
	if tee1.Contains("beta debug") {
		t.Error("tee1 should not have beta messages")
	}
	if tee1.Contains("beta warnf") {
		t.Error("tee1 should not have beta messages")
	}

	// Verify tee2 got all messages
	if !tee2.Contains("alpha trace") {
		t.Error("tee2 should have alpha messages")
	}
	if !tee2.Contains("beta debug") {
		t.Error("tee2 should have beta messages")
	}
	if !tee2.Contains("beta infof") {
		t.Error("tee2 should have beta infof")
	}

	if !panicCalled {
		t.Error("Fatal should have triggered panic handler")
	}
}

// Test Errorf path with filtering to get full coverage
func TestFilterLaneErrorfFullCoverage(t *testing.T) {
	tl := NewTestingLane(context.Background())

	// Test both filter pass and filter block paths
	filter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.Contains(msg, "pass")
	}

	fl := NewFilterLane(tl, filter)

	fl.Errorf("pass %d", 1)
	fl.Errorf("block %d", 2)

	if !tl.Contains("pass 1") {
		t.Error("Errorf should pass when filter matches")
	}
	if tl.Contains("block 2") {
		t.Error("Errorf should block when filter doesn't match")
	}
}

// Test the primary use case: root lane with multiple filtered tees to different lane types
func TestFilterLaneRealWorldUseCase(t *testing.T) {
	// Create temporary files for disk lanes
	auditFile := t.TempDir() + "/audit.log"
	debugFile := t.TempDir() + "/debug.log"

	// Create the root lane (main application lane)
	rootLane := NewLogLane(context.Background())

	// Create an audit log that only captures [AUDIT] messages
	auditDiskLane, err := NewDiskLane(context.Background(), auditFile)
	if err != nil {
		t.Fatalf("Failed to create audit disk lane: %v", err)
	}
	defer auditDiskLane.Close()

	auditFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return strings.HasPrefix(msg, "[AUDIT]")
	}
	filteredAuditLane := NewFilterLane(auditDiskLane, auditFilter)

	// Create a debug log that captures everything
	debugDiskLane, err := NewDiskLane(context.Background(), debugFile)
	if err != nil {
		t.Fatalf("Failed to create debug disk lane: %v", err)
	}
	defer debugDiskLane.Close()

	debugFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return true // Accept everything
	}
	filteredDebugLane := NewFilterLane(debugDiskLane, debugFilter)

	// Create a testing lane that only captures errors and warnings
	testLane := NewTestingLane(context.Background())
	errorWarningFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		return level >= LogLevelWarn
	}
	filteredTestLane := NewFilterLane(testLane, errorWarningFilter)

	// Add all filtered lanes as tees to the root lane
	rootLane.AddTee(filteredAuditLane)
	rootLane.AddTee(filteredDebugLane)
	rootLane.AddTee(filteredTestLane)

	// Simulate application logging with various message types
	rootLane.Info("[AUDIT] user login: alice@example.com")
	rootLane.Info("Application started successfully")
	rootLane.Debug("Loading configuration from /etc/config")
	rootLane.Warn("[AUDIT] failed login attempt: bob@example.com")
	rootLane.Warn("Connection pool nearing capacity")
	rootLane.Error("[AUDIT] security violation: unauthorized access attempt")
	rootLane.Error("Database connection failed")
	rootLane.Info("[AUDIT] user logout: alice@example.com")

	// Verify audit log has only audit messages
	auditContent, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}
	auditLog := string(auditContent)

	if !strings.Contains(auditLog, "[AUDIT] user login: alice@example.com") {
		t.Error("Audit log should contain user login")
	}
	if !strings.Contains(auditLog, "[AUDIT] failed login attempt: bob@example.com") {
		t.Error("Audit log should contain failed login")
	}
	if !strings.Contains(auditLog, "[AUDIT] security violation: unauthorized access attempt") {
		t.Error("Audit log should contain security violation")
	}
	if !strings.Contains(auditLog, "[AUDIT] user logout: alice@example.com") {
		t.Error("Audit log should contain user logout")
	}

	// Verify audit log does NOT have non-audit messages
	if strings.Contains(auditLog, "Application started successfully") {
		t.Error("Audit log should not contain non-audit info messages")
	}
	if strings.Contains(auditLog, "Loading configuration") {
		t.Error("Audit log should not contain debug messages")
	}
	if strings.Contains(auditLog, "Connection pool nearing capacity") {
		t.Error("Audit log should not contain non-audit warning")
	}
	if strings.Contains(auditLog, "Database connection failed") {
		t.Error("Audit log should not contain non-audit errors")
	}

	// Verify debug log has all messages
	debugContent, err := os.ReadFile(debugFile)
	if err != nil {
		t.Fatalf("Failed to read debug log: %v", err)
	}
	debugLog := string(debugContent)

	expectedMessages := []string{
		"[AUDIT] user login: alice@example.com",
		"Application started successfully",
		"Loading configuration from /etc/config",
		"[AUDIT] failed login attempt: bob@example.com",
		"Connection pool nearing capacity",
		"[AUDIT] security violation: unauthorized access attempt",
		"Database connection failed",
		"[AUDIT] user logout: alice@example.com",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(debugLog, msg) {
			t.Errorf("Debug log should contain: %s", msg)
		}
	}

	// Verify test lane has only warnings and errors
	if !testLane.Contains("[AUDIT] failed login attempt: bob@example.com") {
		t.Error("Test lane should contain audit warning")
	}
	if !testLane.Contains("Connection pool nearing capacity") {
		t.Error("Test lane should contain warning")
	}
	if !testLane.Contains("[AUDIT] security violation: unauthorized access attempt") {
		t.Error("Test lane should contain audit error")
	}
	if !testLane.Contains("Database connection failed") {
		t.Error("Test lane should contain error")
	}

	// Verify test lane does NOT have info or debug messages
	if testLane.Contains("Application started successfully") {
		t.Error("Test lane should not contain info messages")
	}
	if testLane.Contains("Loading configuration") {
		t.Error("Test lane should not contain debug messages")
	}

	// Verify the counts make sense
	auditCount := strings.Count(auditLog, "[AUDIT]")
	if auditCount != 4 {
		t.Errorf("Audit log should have exactly 4 audit messages, got %d", auditCount)
	}

	debugLineCount := strings.Count(debugLog, "\n")
	if debugLineCount < 8 {
		t.Errorf("Debug log should have at least 8 lines, got %d", debugLineCount)
	}
}
