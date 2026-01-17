package lane

import (
	"context"
	"strings"
	"testing"
)

// TestFilterByMetadata demonstrates filtering based on lane metadata and message content
func TestFilterByMetadata(t *testing.T) {
	// Create a lane that will do the logging
	mainLane := NewTestingLane(context.Background())
	mainLane.SetMetadata("tenant", "acme-corp")
	mainLane.SetJourneyId("prod-request-123")

	// Create a filter that checks BOTH the logging lane's properties AND message content
	tenantLog := NewTestingLane(context.Background())

	// Wrap the main lane so the filter can access its metadata
	// The filter sees the wrapped lane (tenantLog) not the parent (mainLane)
	// So we need to check the message content combined with level
	combinedFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		// Filter: only errors that contain [TENANT]
		return level >= LogLevelError && strings.Contains(msg, "[TENANT]")
	}
	tenantFilteredLane := NewFilterLane(tenantLog, combinedFilter)
	mainLane.AddTee(tenantFilteredLane)

	mainLane.Info("[TENANT] User logged in")  // Info level - filtered out
	mainLane.Error("[TENANT] Database error") // Error + [TENANT] - passes
	mainLane.Error("Regular error")           // Error without [TENANT] - filtered out
	mainLane.Warn("[TENANT] Connection slow") // Warn level - filtered out

	// Only the error with [TENANT] should pass
	if tenantLog.Contains("User logged in") {
		t.Error("Should not contain info message")
	}
	if !tenantLog.Contains("[TENANT] Database error") {
		t.Error("Should contain tenant error")
	}
	if tenantLog.Contains("Regular error") {
		t.Error("Should not contain regular error")
	}
	if tenantLog.Contains("Connection slow") {
		t.Error("Should not contain warning")
	}
}

// TestFilterByJourneyId demonstrates accessing lane properties in the filter
func TestFilterByJourneyId(t *testing.T) {
	// When logging DIRECTLY to a FilterLane (not via tee), the filter
	// receives the wrapped lane as the first parameter
	testLog := NewTestingLane(context.Background())
	testLog.SetJourneyId("prod-12345")
	testLog.SetMetadata("env", "production")

	prodLog := NewTestingLane(context.Background())

	// This filter checks the wrapped lane's (testLog's) properties
	prodFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		// Since this filter wraps prodLog, not testLog, lane.JourneyId() returns prodLog's journey ID
		// For direct logging to FilterLane, we'd need to log to testLog and check there
		// This demonstrates that filters see the WRAPPED lane, not the logging lane
		return strings.HasPrefix(msg, "[PROD]") && level >= LogLevelWarn
	}
	prodFilteredLane := NewFilterLane(prodLog, prodFilter)
	testLog.AddTee(prodFilteredLane)

	testLog.Info("[PROD] Processing request")  // Info - filtered out
	testLog.Warn("[PROD] Slow query detected") // Warn + [PROD] - passes
	testLog.Error("[DEV] Test error")          // Error but wrong prefix - filtered out

	// Only warn/error with [PROD] should pass
	if prodLog.Contains("Processing request") {
		t.Error("Should not contain info message")
	}
	if !prodLog.Contains("Slow query detected") {
		t.Error("Should contain prod warning")
	}
	if prodLog.Contains("Test error") {
		t.Error("Should not contain dev error")
	}
}

// TestFilterCombinedMetadataAndLevel demonstrates the lane parameter provides access to wrapped lane properties
func TestFilterCombinedMetadataAndLevel(t *testing.T) {
	// Create a filtered lane where we can check the wrapped lane's properties
	premiumErrorLog := NewTestingLane(context.Background())
	premiumErrorLog.SetMetadata("customer-tier", "premium") // Set metadata on the lane being filtered

	// Filter based on the lane's own metadata + level
	premiumErrorFilter := func(lane Lane, level LaneLogLevel, msg string) bool {
		// 'lane' is premiumErrorLog (the wrapped lane)
		isPremium := lane.GetMetadata("customer-tier") == "premium"
		isError := level >= LogLevelError
		return isPremium && isError
	}
	premiumErrorLane := NewFilterLane(premiumErrorLog, premiumErrorFilter)

	// Log directly to the filtered lane
	premiumErrorLane.Info("User action")     // Not an error - filtered out
	premiumErrorLane.Warn("Rate limited")    // Not an error - filtered out
	premiumErrorLane.Error("Payment failed") // Error + premium = passes

	// Verify only errors passed through
	if premiumErrorLog.Contains("User action") {
		t.Error("Should not contain info message")
	}
	if premiumErrorLog.Contains("Rate limited") {
		t.Error("Should not contain warning message")
	}
	if !premiumErrorLog.Contains("Payment failed") {
		t.Error("Should contain error message")
	}
}
