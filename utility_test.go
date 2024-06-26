package lane

import (
	"bytes"
	"log"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"
)

type (
	testStruct struct {
		a int
		b int
	}
	testStruct2 struct {
		name string
		Link any
	}
	testStruct3 struct {
		m map[string]*testStruct2
	}

	testRecursive struct {
		name string
		next *testRecursive
	}

	testRecursiveAny struct {
		name string
		next any
	}
)

var objLineExp = regexp.MustCompile(`\d{4}\/\d\d\/\d\d \d\d:\d\d:\d\d [A-Z]+ \{[a-z0-9]{10}\} (.*)\n`)

func testExpectedStdout(t *testing.T, buf *bytes.Buffer, expected []string) {
	capture := buf.String()

	matches := objLineExp.FindAllStringSubmatch(capture, -1)
	for i, expectation := range expected {
		if i >= len(matches) {
			break
		}
		match := matches[i]

		var addrText string
		matchText := match[1]
		addr := strings.Index(matchText, `"":"Address: 0x`)
		if addr >= 0 {
			addr += 15
			length := strings.Index(matchText[addr:], `"`)
			if length > 0 {
				addrText = "0x" + matchText[addr:addr+length]
			}
		}

		replaced := strings.ReplaceAll(expectation, "**addr**", addrText)
		if matchText != replaced {
			t.Errorf("expected '%s', got '%s'", replaced, matchText)
		}
	}

	if len(expected) != len(matches) {
		t.Fatalf("expected %d lines, got %d", len(expected), len(matches))
	}
}

func TestLogObject(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	v1 := int(123)
	LogObject(l, LogLevelTrace, "trace", v1)
	v1++
	LogObject(l, LogLevelDebug, "debug", v1)
	v1++
	LogObject(l, LogLevelInfo, "info", v1)
	v1++
	LogObject(l, LogLevelWarn, "warn", v1)
	v1++
	LogObject(l, LogLevelError, "error", v1)
	v1++

	wg := setTestPanicHandler(l)
	go func() {
		LogObject(l, LogLevelFatal, "fatal", v1)
		panic("unreachable")
	}()
	wg.Wait()

	testExpectedStdout(t, &buf, []string{
		"trace: 123",
		"debug: 124",
		"info: 125",
		"warn: 126",
		"error: 127",
		"fatal: 128",
	})
}

func TestLogLaneObject(t *testing.T) {
	l := NewLogLane(nil)
	l2 := NewLogLane(nil)
	l.AddTee(l2)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	v1 := int(123)
	l.TraceObject("trace", v1)
	v1++
	l.DebugObject("debug", v1)
	v1++
	l.InfoObject("info", v1)
	v1++
	l.WarnObject("warn", v1)
	v1++
	l.ErrorObject("error", v1)
	v1++
	l.PreFatalObject("pre-fatal", v1)
	v1++

	wg := setTestPanicHandler(l)
	go func() {
		l.FatalObject("fatal", v1)
		panic("unreachable")
	}()
	wg.Wait()

	testExpectedStdout(t, &buf, []string{
		"trace: 123",
		"trace: 123",
		"debug: 124",
		"debug: 124",
		"info: 125",
		"info: 125",
		"warn: 126",
		"warn: 126",
		"error: 127",
		"error: 127",
		"pre-fatal: 128",
		"pre-fatal: 128",
		"fatal: 129",
		"fatal: 129",
	})
}

func TestNullLaneObject(t *testing.T) {
	l := NewNullLane(nil)
	l2 := NewLogLane(nil)
	l.AddTee(l2)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	v1 := int(123)
	l.TraceObject("trace", v1)
	v1++
	l.DebugObject("debug", v1)
	v1++
	l.InfoObject("info", v1)
	v1++
	l.WarnObject("warn", v1)
	v1++
	l.ErrorObject("error", v1)
	v1++
	l.PreFatalObject("pre-fatal", v1)
	v1++

	wg := setTestPanicHandler(l)
	go func() {
		l.FatalObject("fatal", v1)
		panic("unreachable")
	}()
	wg.Wait()

	testExpectedStdout(t, &buf, []string{
		"trace: 123",
		"debug: 124",
		"info: 125",
		"warn: 126",
		"error: 127",
		"pre-fatal: 128",
		"fatal: 129",
	})
}

func TestTestingLaneObject(t *testing.T) {
	l := NewTestingLane(nil)
	l2 := NewLogLane(nil)
	l.AddTee(l2)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	v1 := int(123)
	l.TraceObject("trace", v1)
	v1++
	l.DebugObject("debug", v1)
	v1++
	l.InfoObject("info", v1)
	v1++
	l.WarnObject("warn", v1)
	v1++
	l.ErrorObject("error", v1)
	v1++
	l.PreFatalObject("pre-fatal", v1)
	v1++

	wg := setTestPanicHandler(l)
	go func() {
		l.FatalObject("fatal", v1)
		panic("unreachable")
	}()
	wg.Wait()

	testExpectedStdout(t, &buf, []string{
		"trace: 123",
		"debug: 124",
		"info: 125",
		"warn: 126",
		"error: 127",
		"pre-fatal: 128",
		"fatal: 129",
	})

	verified := l.VerifyEventText(`TRACE	trace: 123
DEBUG	debug: 124
INFO	info: 125
WARN	warn: 126
ERROR	error: 127
FATAL	pre-fatal: 128
FATAL	fatal: 129`)

	if !verified {
		t.Error("test lane does not have expected events")
	}
}

func TestLogObjectValueTypes(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	l.InfoObject("bool", true)
	l.InfoObject("int", int(1))
	l.InfoObject("int8", int8(-8))
	l.InfoObject("int16", int16(-16))
	l.InfoObject("int32", int32(-32))
	l.InfoObject("int64", int64(-64))
	l.InfoObject("uint", uint(2))
	l.InfoObject("uint8", uint8(8))
	l.InfoObject("uint16", uint16(16))
	l.InfoObject("uint32", uint32(32))
	l.InfoObject("uint64", uint64(64))
	l.InfoObject("uintptr", uintptr(123))
	l.InfoObject("float32", float32(32.32))
	l.InfoObject("float64", float64(64.64))
	l.InfoObject("complex64", complex64(complex(64, 0.64)))
	l.InfoObject("complex128", complex(128, 0.128))
	l.InfoObject("string", "hello")

	testExpectedStdout(t, &buf, []string{
		"bool: true",
		"int: 1",
		"int8: -8",
		"int16: -16",
		"int32: -32",
		"int64: -64",
		"uint: 2",
		"uint8: 8",
		"uint16: 16",
		"uint32: 32",
		"uint64: 64",
		"uintptr: 123",
		"float32: 32.32",
		"float64: 64.64",
		`complex64: "(64+0.64i)"`,
		`complex128: "(128+0.128i)"`,
		`string: "hello"`,
	})
}

func TestLogObjectNonValues(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	ch := make(chan bool)

	l.InfoObject("chan", ch)
	l.InfoObject("func", TestLogObjectNonValues)

	testExpectedStdout(t, &buf, []string{
		`chan: "chan bool"`,
		`func: "github.com/jimsnab/go-lane.TestLogObjectNonValues"`,
	})
}

func TestLogObjectSlice(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	a := []string{"cat", "dog"}
	l.InfoObject("slice", a)

	testExpectedStdout(t, &buf, []string{
		`slice: ["cat","dog"]`,
	})
}

func TestLogObjectArray(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	a := [2]string{"cat", "dog"}
	l.InfoObject("array", a)

	testExpectedStdout(t, &buf, []string{
		`array: ["cat","dog"]`,
	})
}

func TestLogObjectMap(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	m := map[string]int{"cat": 1, "dog": 2}
	l.InfoObject("map", m)

	testExpectedStdout(t, &buf, []string{
		`map: {"cat":1,"dog":2}`,
	})
}

func TestLogObjectMap2(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	m := map[testStruct]int{{a: 10, b: 20}: 1, {a: 20, b: 30}: 2}
	l.InfoObject("map", m)

	testExpectedStdout(t, &buf, []string{
		`map: {"map[a:10 b:20]":1,"map[a:20 b:30]":2}`,
	})
}

func TestLogObjectMap3(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	m := map[int]testStruct{1: {a: 10, b: 20}, 2: {a: 20, b: 30}}
	l.InfoObject("map", m)

	testExpectedStdout(t, &buf, []string{
		`map: {"1":{"a":10,"b":20},"2":{"a":20,"b":30}}`,
	})
}

func TestLogObjectStruct(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	s := testStruct{a: 10, b: 20}
	l.InfoObject("struct", s)
	l.InfoObject("ptr-struct", &s)

	testExpectedStdout(t, &buf, []string{
		`struct: {"a":10,"b":20}`,
		`ptr-struct: {"a":10,"b":20}`,
	})
}

func TestLogObjectInterface(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	a := any(1)
	l.InfoObject("int-any", a)

	b := any([]string{"one", "two"})
	l.InfoObject("slice-any", b)

	testExpectedStdout(t, &buf, []string{
		`int-any: 1`,
		`slice-any: ["one","two"]`,
	})
}

func TestLogObjectComposite(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	v := []any{}
	v = append(v, &testStruct2{name: "parent", Link: &testStruct2{name: "child"}})
	l.InfoObject("composite", v)

	testExpectedStdout(t, &buf, []string{
		`composite: [{"Link":{"Link":null,"name":"child"},"name":"parent"}]`,
	})
}

func TestLogObjectComposite2(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	v := testStruct3{m: map[string]*testStruct2{}}
	v2 := testStruct2{name: "child"}
	v3 := testStruct3{m: map[string]*testStruct2{}}
	v2.Link = v3
	v3.m["child"] = &testStruct2{name: "leaf"}
	v.m["root"] = &v2

	l.InfoObject("composite", v)

	testExpectedStdout(t, &buf, []string{
		`composite: {"m":{"root":{"Link":{"m":{"child":{"Link":null,"name":"leaf"}}},"name":"child"}}}`,
	})
}

func TestLogObjectComposite3(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	m := map[string]any{}

	m["chan"] = make(chan bool)
	m["fn"] = TestLogObjectComposite3
	m["array"] = [2]int{}
	m["slice"] = []uint{}

	l.InfoObject("composite", m)

	testExpectedStdout(t, &buf, []string{
		`composite: {"array":[0,0],"chan":"chan bool","fn":"github.com/jimsnab/go-lane.TestLogObjectComposite3","slice":[]}`,
	})
}

func TestLogObjectByteSlice(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	a := []byte("chicken\tcow\n\"lamb\" \\\"horse\\\"")
	l.InfoObject("slice", a)

	testExpectedStdout(t, &buf, []string{
		`slice: "chicken\tcow\n\"lamb\" \\\"horse\\\""`,
	})
}

func TestLogObjectRecursive(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	n1 := testRecursive{name: "n1"}
	n2 := testRecursive{name: "n2..n1", next: &n1}
	n3 := testRecursive{name: "n3..n3"}
	n3.next = &n3

	l.InfoObject("n1", &n1)
	l.InfoObject("n2", &n2)
	l.InfoObject("n3", &n3)

	testExpectedStdout(t, &buf, []string{
		`n1: {"name":"n1","next":null}`,
		`n2: {"name":"n2..n1","next":{"name":"n1","next":null}}`,
		`n3: {"":"Address: **addr**","name":"n3..n3","next":"(pointer: **addr**)"}`,
	})
}

func TestLogObjectRecursive2(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	n1 := testRecursive{name: "n1..n2"}
	n2 := testRecursive{name: "n2..n3"}
	n3 := testRecursive{name: "n3..n1"}
	n1.next = &n2
	n2.next = &n3
	n3.next = &n1

	l.InfoObject("n1", &n1)
	l.InfoObject("n2", &n2)
	l.InfoObject("n3", &n3)

	testExpectedStdout(t, &buf, []string{
		`n1: {"":"Address: **addr**","name":"n1..n2","next":{"name":"n2..n3","next":{"name":"n3..n1","next":"(pointer: **addr**)"}}}`,
		`n2: {"":"Address: **addr**","name":"n2..n3","next":{"name":"n3..n1","next":{"name":"n1..n2","next":"(pointer: **addr**)"}}}`,
		`n3: {"":"Address: **addr**","name":"n3..n1","next":{"name":"n1..n2","next":{"name":"n2..n3","next":"(pointer: **addr**)"}}}`,
	})
}

func TestLogObjectRecursiveAny(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	n1 := testRecursiveAny{name: "n1..n1"}
	n1.next = &n1

	l.InfoObject("n1", &n1)

	testExpectedStdout(t, &buf, []string{
		`n1: {"":"Address: **addr**","name":"n1..n1","next":"(pointer: **addr**)"}`,
	})
}

func TestLogObjectInf(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	a := math.Inf(1)
	l.InfoObject("inf", a)

	a = math.Inf(-1)
	l.InfoObject("inf", a)

	testExpectedStdout(t, &buf, []string{
		`inf: "+Inf"`,
		`inf: "-Inf"`,
	})
}

func TestLogObjectComplex(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	a := complex64(1)
	l.InfoObject("c64", a)

	b := complex128(10 + .3i)
	l.InfoObject("c128", b)

	testExpectedStdout(t, &buf, []string{
		`c64: "(1+0i)"`,
		`c128: "(10+0.3i)"`,
	})
}

func TestLogObjectLargeByteArray(t *testing.T) {
	l := NewLogLane(nil)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() { log.SetOutput(os.Stderr) }()

	a := make([]byte, 2048)
	for i := range 2048 {
		a[i] = byte(i % 256)
	}
	l.InfoObject("array", a)

	testExpectedStdout(t, &buf, []string{
		`array: "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJTVFVWV1h` +
			`ZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsbKztLW2t7` +
			`i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/wABAgMEBQYHCAkKCwwNDg8QERITFBUWF` +
			`xgZGhscHR4fICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj9AQUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVpbXF1eX2BhYmNkZWZnaGlqa2xtbm9wcXJzdHV2` +
			`d3h5ent8fX5/gIGCg4SFhoeIiYqLjI2Oj5CRkpOUlZaXmJmam5ydnp+goaKjpKWmp6ipqqusra6vsLGys7S1tre4ubq7vL2+v8DBwsPExcbHyMnKy8zNzs/Q0dLT1NX` +
			`W19jZ2tvc3d7f4OHi4+Tl5ufo6err7O3u7/Dx8vP09fb3+Pn6+/z9/v8AAQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyAhIiMkJSYnKCkqKywtLi8wMTIzND` +
			`U2Nzg5Ojs8PT4/QEFCQ0RFRkdISUpLTE1OT1BRUlNUVVZXWFlaW1xdXl9gYWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXp7fH1+f4CBgoOEhYaHiImKi4yNjo+QkZKTl` +
			`JWWl5iZmpucnZ6foKGio6SlpqeoqaqrrK2ur7CxsrO0tba3uLm6u7y9vr/AwcLDxMXGx8jJysvMzc7P0NHS09TV1tfY2drb3N3e3+Dh4uPk5ebn6Onq6+zt7u/w8fLz` +
			`9PX29/j5+vv8/f7/AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0xNTk9QUVJ` +
			`TVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6ytrq+wsb` +
			`KztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/wABAgMEBQYHCAkKCwwNDg8QE` +
			`RITFBUWFxgZGhscHR4fICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj9AQUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVpbXF1eX2BhYmNkZWZnaGlqa2xtbm9w` +
			`cXJzdHV2d3h5ent8fX5/gIGCg4SFhoeIiYqLjI2Oj5CRkpOUlZaXmJmam5ydnp+goaKjpKWmp6ipqqusra6vsLGys7S1tre4ubq7vL2+v8DBwsPExcbHyMnKy8zNzs/` +
			`Q0dLT1NXW19jZ2tvc3d7f4OHi4+Tl5ufo6err7O3u7/Dx8vP09fb3+Pn6+/z9/v8AAQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyAhIiMkJSYnKCkqKywtLi` +
			`8wMTIzNDU2Nzg5Ojs8PT4/QEFCQ0RFRkdISUpLTE1OT1BRUlNUVVZXWFlaW1xdXl9gYWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXp7fH1+f4CBgoOEhYaHiImKi4yNj` +
			`o+QkZKTlJWWl5iZmpucnZ6foKGio6SlpqeoqaqrrK2ur7CxsrO0tba3uLm6u7y9vr/AwcLDxMXGx8jJysvMzc7P0NHS09TV1tfY2drb3N3e3+Dh4uPk5ebn6Onq6+zt` +
			`7u/w8fLz9PX29/j5+vv8/f7/AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8gISIjJCUmJygpKissLS4vMDEyMzQ1Njc4OTo7PD0+P0BBQkNERUZHSElKS0x` +
			`NTk9QUVJTVFVWV1hZWltcXV5fYGFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6e3x9fn+AgYKDhIWGh4iJiouMjY6PkJGSk5SVlpeYmZqbnJ2en6ChoqOkpaanqKmqq6` +
			`ytrq+wsbKztLW2t7i5uru8vb6/wMHCw8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t/g4eLj5OXm5+jp6uvs7e7v8PHy8/T19vf4+fr7/P3+/wABAgMEBQYHCAkKC` +
			`wwNDg8QERITFBUWFxgZGhscHR4fICEiIyQlJicoKSorLC0uLzAxMjM0NTY3ODk6Ozw9Pj9AQUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVpbXF1eX2BhYmNkZWZnaGlq` +
			`a2xtbm9wcXJzdHV2d3h5ent8fX5/gIGCg4SFhoeIiYqLjI2Oj5CRkpOUlZaXmJmam5ydnp+goaKjpKWmp6ipqqusra6vsLGys7S1tre4ubq7vL2+v8DBwsPExcbHyMn` +
			`Ky8zNzs/Q0dLT1NXW19jZ2tvc3d7f4OHi4+Tl5ufo6err7O3u7/Dx8vP09fb3+Pn6+/z9/v8="`,
	})
}
