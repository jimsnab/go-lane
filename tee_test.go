package lane

import (
	"context"
	"testing"
	"time"
)

func TestTeeLog(t *testing.T) {
	tl := NewTestingLane(context.Background())

	ll := NewLogLane(context.Background())
	ll.AddTee(tl)

	ll.Trace("trace", 1)
	ll.Tracef("%s %d", "tracef", 1)
	ll.Debug("debug", 1)
	ll.Debugf("%s %d", "debugf", 1)
	ll.Info("info", 1)
	ll.Infof("%s %d", "infof", 1)
	ll.Warn("warn", 1)
	ll.Warnf("%s %d", "warnf", 1)
	ll.Error("error", 1)
	ll.Errorf("%s %d", "errorf", 1)
	ll.PreFatal("fatal", 1)
	ll.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tl.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeLogDerive(t *testing.T) {
	tl := NewTestingLane(context.Background())

	ll := NewLogLane(context.Background())
	ll.AddTee(tl)

	ll.Trace("trace", 1)
	ll.Tracef("%s %d", "tracef", 1)

	ll2 := ll.Derive()
	ll2.Debug("debug", 1)
	ll2.Debugf("%s %d", "debugf", 1)

	ll3, _ := ll2.DeriveWithCancel()
	ll3.Info("info", 1)
	ll3.Infof("%s %d", "infof", 1)

	ll4, _ := ll3.DeriveWithDeadline(time.Now().Add(time.Hour))
	ll4.Warn("warn", 1)
	ll4.Warnf("%s %d", "warnf", 1)

	ll5, _ := ll4.DeriveWithTimeout(time.Hour)
	ll5.Error("error", 1)
	ll5.Errorf("%s %d", "errorf", 1)
	ll5.PreFatal("fatal", 1)
	ll5.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tl.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeLogDouble(t *testing.T) {
	tl1 := NewTestingLane(context.Background())
	tl2 := NewTestingLane(context.Background())

	ll := NewLogLane(context.Background())
	ll.AddTee(tl1)
	ll.AddTee(tl2)

	ll.Trace("trace", 1)
	ll.Tracef("%s %d", "tracef", 1)
	ll.Debug("debug", 1)
	ll.Debugf("%s %d", "debugf", 1)
	ll.Info("info", 1)
	ll.Infof("%s %d", "infof", 1)
	ll.Warn("warn", 1)
	ll.Warnf("%s %d", "warnf", 1)
	ll.Error("error", 1)
	ll.Errorf("%s %d", "errorf", 1)
	ll.PreFatal("fatal", 1)
	ll.PreFatalf("%s %d", "fatalf", 1)

	ll.RemoveTee(tl2)
	ll.PreFatal("fatal", 1)
	ll.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tl1.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
	if !tl2.VerifyEvents(events[:12]) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeNull(t *testing.T) {
	tl := NewTestingLane(context.Background())

	nl := NewNullLane(context.Background())
	nl.AddTee(tl)

	nl.Trace("trace", 1)
	nl.Tracef("%s %d", "tracef", 1)
	nl.Debug("debug", 1)
	nl.Debugf("%s %d", "debugf", 1)
	nl.Info("info", 1)
	nl.Infof("%s %d", "infof", 1)
	nl.Warn("warn", 1)
	nl.Warnf("%s %d", "warnf", 1)
	nl.Error("error", 1)
	nl.Errorf("%s %d", "errorf", 1)
	nl.PreFatal("fatal", 1)
	nl.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tl.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeNullDerive(t *testing.T) {
	tl := NewTestingLane(context.Background())

	nl := NewNullLane(context.Background())
	nl.AddTee(tl)

	nl.Trace("trace", 1)
	nl.Tracef("%s %d", "tracef", 1)

	nl2 := nl.Derive()
	nl2.Debug("debug", 1)
	nl2.Debugf("%s %d", "debugf", 1)

	nl3, _ := nl2.DeriveWithCancel()
	nl3.Info("info", 1)
	nl3.Infof("%s %d", "infof", 1)

	nl4, _ := nl.DeriveWithDeadline(time.Now().Add(time.Hour))
	nl4.Warn("warn", 1)
	nl4.Warnf("%s %d", "warnf", 1)

	nl5, _ := nl4.DeriveWithTimeout(time.Hour)
	nl5.Error("error", 1)
	nl5.Errorf("%s %d", "errorf", 1)
	nl5.PreFatal("fatal", 1)
	nl5.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tl.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeNullDouble(t *testing.T) {
	tl1 := NewTestingLane(context.Background())
	tl2 := NewTestingLane(context.Background())

	nl := NewNullLane(context.Background())
	nl.AddTee(tl1)
	nl.AddTee(tl2)

	nl.Trace("trace", 1)
	nl.Tracef("%s %d", "tracef", 1)
	nl.Debug("debug", 1)
	nl.Debugf("%s %d", "debugf", 1)
	nl.Info("info", 1)
	nl.Infof("%s %d", "infof", 1)
	nl.Warn("warn", 1)
	nl.Warnf("%s %d", "warnf", 1)
	nl.Error("error", 1)
	nl.Errorf("%s %d", "errorf", 1)
	nl.PreFatal("fatal", 1)
	nl.PreFatalf("%s %d", "fatalf", 1)

	nl.RemoveTee(tl2)
	nl.PreFatal("fatal", 1)
	nl.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tl1.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
	if !tl2.VerifyEvents(events[:12]) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeTest(t *testing.T) {
	tl := NewTestingLane(context.Background())

	tl2 := NewTestingLane(context.Background())
	tl2.AddTee(tl)

	tl2.Trace("trace", 1)
	tl2.Tracef("%s %d", "tracef", 1)
	tl2.Debug("debug", 1)
	tl2.Debugf("%s %d", "debugf", 1)
	tl2.Info("info", 1)
	tl2.Infof("%s %d", "infof", 1)
	tl2.Warn("warn", 1)
	tl2.Warnf("%s %d", "warnf", 1)
	tl2.Error("error", 1)
	tl2.Errorf("%s %d", "errorf", 1)
	tl2.PreFatal("fatal", 1)
	tl2.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tl.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeTestDerive(t *testing.T) {
	tlv := NewTestingLane(context.Background())

	tl := NewTestingLane(context.Background())
	tl.AddTee(tlv)

	tl.Trace("trace", 1)
	tl.Tracef("%s %d", "tracef", 1)

	tl2 := tl.Derive()
	tl2.Debug("debug", 1)
	tl2.Debugf("%s %d", "debugf", 1)

	tl3, _ := tl2.DeriveWithCancel()
	tl3.Info("info", 1)
	tl3.Infof("%s %d", "infof", 1)

	tl4, _ := tl.DeriveWithDeadline(time.Now().Add(time.Hour))
	tl4.Warn("warn", 1)
	tl4.Warnf("%s %d", "warnf", 1)

	tl5, _ := tl3.DeriveWithTimeout(time.Hour)
	tl5.Error("error", 1)
	tl5.Errorf("%s %d", "errorf", 1)
	tl5.PreFatal("fatal", 1)
	tl5.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tlv.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeTestDouble(t *testing.T) {
	tl1 := NewTestingLane(context.Background())
	tl2 := NewTestingLane(context.Background())

	tl := NewTestingLane(context.Background())
	tl.AddTee(tl1)
	tl.AddTee(tl2)

	tl.Trace("trace", 1)
	tl.Tracef("%s %d", "tracef", 1)
	tl.Debug("debug", 1)
	tl.Debugf("%s %d", "debugf", 1)
	tl.Info("info", 1)
	tl.Infof("%s %d", "infof", 1)
	tl.Warn("warn", 1)
	tl.Warnf("%s %d", "warnf", 1)
	tl.Error("error", 1)
	tl.Errorf("%s %d", "errorf", 1)
	tl.PreFatal("fatal", 1)
	tl.PreFatalf("%s %d", "fatalf", 1)

	tl.RemoveTee(tl2)
	tl.PreFatal("fatal", 1)
	tl.PreFatalf("%s %d", "fatalf", 1)

	events := []*LaneEvent{}
	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 1"})
	events = append(events, &LaneEvent{Level: "TRACE", Message: "tracef 1"})

	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debug 1"})
	events = append(events, &LaneEvent{Level: "DEBUG", Message: "debugf 1"})

	events = append(events, &LaneEvent{Level: "INFO", Message: "info 1"})
	events = append(events, &LaneEvent{Level: "INFO", Message: "infof 1"})

	events = append(events, &LaneEvent{Level: "WARN", Message: "warn 1"})
	events = append(events, &LaneEvent{Level: "WARN", Message: "warnf 1"})

	events = append(events, &LaneEvent{Level: "ERROR", Message: "error 1"})
	events = append(events, &LaneEvent{Level: "ERROR", Message: "errorf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatal 1"})
	events = append(events, &LaneEvent{Level: "FATAL", Message: "fatalf 1"})

	if !tl1.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
	if !tl2.VerifyEvents(events[:12]) {
		t.Errorf("Test events don't match")
	}
}
