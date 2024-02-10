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

	if tl.GetMetadata("test") != "" {
		t.Fatal("metadata unexpected")
	}

	ll.SetMetadata("test", "1")

	if tl.GetMetadata("test") != "1" {
		t.Fatal("metadata expected")
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

	if tl.GetMetadata("test") != "" {
		t.Fatal("metadata unexpected")
	}

	nl.SetMetadata("test", "1")

	if tl.GetMetadata("test") != "1" {
		t.Fatal("metadata expected")
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

func TestTeeTestDerive1(t *testing.T) {
	tlv := NewTestingLane(context.Background())

	tl := NewTestingLane(context.Background())
	tl.AddTee(tlv)

	tl.Trace("trace", 1)
	tl.Tracef("%s %d", "tracef", 1)

	tl2 := tl.Derive()
	tl2.Debug("debug", 1)
	tl2.Debugf("%s %d", "debugf", 1)

	tl3, cf := tl2.DeriveWithCancel()
	tl3.Info("info", 1)
	tl3.Infof("%s %d", "infof", 1)
	cf() // free chan resource

	tl4, cf := tl.DeriveWithDeadline(time.Now().Add(time.Hour))
	tl4.Warn("warn", 1)
	tl4.Warnf("%s %d", "warnf", 1)
	cf() // free chan resource

	tl5, cf := tl3.DeriveWithTimeout(time.Hour)
	tl5.Error("error", 1)
	tl5.Errorf("%s %d", "errorf", 1)
	tl5.PreFatal("fatal", 1)
	tl5.PreFatalf("%s %d", "fatalf", 1)
	cf() // free chan resource

	tl6 := tl5.DeriveReplaceContext(context.Background())
	tl6.Trace("trace", 2)

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

	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 2"})

	if !tlv.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeTestDerive2(t *testing.T) {
	tlv := NewTestingLane(context.Background())

	tl := NewNullLane(context.Background())
	tl.AddTee(tlv)

	tl.Trace("trace", 1)
	tl.Tracef("%s %d", "tracef", 1)

	tl2 := tl.Derive()
	tl2.Debug("debug", 1)
	tl2.Debugf("%s %d", "debugf", 1)

	tl3, cf := tl2.DeriveWithCancel()
	tl3.Info("info", 1)
	tl3.Infof("%s %d", "infof", 1)
	cf() // free chan resource

	tl4, cf := tl.DeriveWithDeadline(time.Now().Add(time.Hour))
	tl4.Warn("warn", 1)
	tl4.Warnf("%s %d", "warnf", 1)
	cf() // free chan resource

	tl5, cf := tl3.DeriveWithTimeout(time.Hour)
	tl5.Error("error", 1)
	tl5.Errorf("%s %d", "errorf", 1)
	tl5.PreFatal("fatal", 1)
	tl5.PreFatalf("%s %d", "fatalf", 1)
	cf() // free chan resource

	tl6 := tl5.DeriveReplaceContext(context.Background())
	tl6.Trace("trace", 2)

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

	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 2"})

	if !tlv.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeTestDerive3(t *testing.T) {
	tlv := NewTestingLane(context.Background())

	tl := NewLogLane(context.Background())
	tl.AddTee(tlv)

	tl.Trace("trace", 1)
	tl.Tracef("%s %d", "tracef", 1)

	tl2 := tl.Derive()
	tl2.Debug("debug", 1)
	tl2.Debugf("%s %d", "debugf", 1)

	tl3, cf := tl2.DeriveWithCancel()
	tl3.Info("info", 1)
	tl3.Infof("%s %d", "infof", 1)
	cf() // free chan resource

	tl4, cf := tl.DeriveWithDeadline(time.Now().Add(time.Hour))
	tl4.Warn("warn", 1)
	tl4.Warnf("%s %d", "warnf", 1)
	cf() // free chan resource

	tl5, cf := tl3.DeriveWithTimeout(time.Hour)
	tl5.Error("error", 1)
	tl5.Errorf("%s %d", "errorf", 1)
	tl5.PreFatal("fatal", 1)
	tl5.PreFatalf("%s %d", "fatalf", 1)
	cf() // free chan resource

	tl6 := tl5.DeriveReplaceContext(context.Background())
	tl6.Trace("trace", 2)

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

	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 2"})

	if !tlv.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}
}

func TestTeeTestDerive4(t *testing.T) {
	tlv := NewTestingLane(context.Background())

	tl, err := NewDiskLane(context.Background(), "test.log")
	if err != nil {
		t.Fatal(err)
	}
	tl.AddTee(tlv)

	tl.Trace("trace", 1)
	tl.Tracef("%s %d", "tracef", 1)

	tl2 := tl.Derive()
	tl2.Debug("debug", 1)
	tl2.Debugf("%s %d", "debugf", 1)

	tl3, cf := tl2.DeriveWithCancel()
	tl3.Info("info", 1)
	tl3.Infof("%s %d", "infof", 1)
	cf() // free chan resource

	tl4, cf := tl.DeriveWithDeadline(time.Now().Add(time.Hour))
	tl4.Warn("warn", 1)
	tl4.Warnf("%s %d", "warnf", 1)
	cf() // free chan resource

	tl5, cf := tl3.DeriveWithTimeout(time.Hour)
	tl5.Error("error", 1)
	tl5.Errorf("%s %d", "errorf", 1)
	tl5.PreFatal("fatal", 1)
	tl5.PreFatalf("%s %d", "fatalf", 1)
	cf() // free chan resource

	tl6 := tl5.DeriveReplaceContext(context.Background())
	tl6.Trace("trace", 2)

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

	events = append(events, &LaneEvent{Level: "TRACE", Message: "trace 2"})

	if !tlv.VerifyEvents(events) {
		t.Errorf("Test events don't match")
	}

	if tlv.GetMetadata("test") != "" {
		t.Fatal("metadata unexpected")
	}

	tl.SetMetadata("test", "1")

	if tlv.GetMetadata("test") != "1" {
		t.Fatal("metadata expected")
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

func TestTestingLaneMetadata(t *testing.T) {
	tl1 := NewTestingLane(context.Background())

	tl2 := NewTestingLane(context.Background())
	tl1.AddTee(tl2)

	if tl1.GetMetadata("test") != "" {
		t.Fatal("unexpected existing metadata")
	}

	if tl2.GetMetadata("test") != "" {
		t.Fatal("unexpected existing metadata")
	}

	tl1.SetMetadata("main", "tee")
	if tl1.GetMetadata("main") != "tee" {
		t.Fatal("expected main testing lane metadata")
	}
	if tl2.GetMetadata("main") != "tee" {
		t.Fatal("expected teed testing lane metadata")
	}

	tl2.SetMetadata("teed", "tee")
	if tl1.GetMetadata("main") != "tee" {
		t.Fatal("expected main testing lane metadata")
	}
	if tl1.GetMetadata("teed") != "" {
		t.Fatal("didn't expect teed lane metadata in main lane")
	}
	if tl2.GetMetadata("teed") != "tee" {
		t.Fatal("expected teed testing lane metadata")
	}
}
