package lane

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	"github.com/google/uuid"
)

type (
	asciiSequence []byte
	recursionType int
)

const (
	recursionNone recursionType = iota
	recursionPossible
	recursionFirst
	recursionWasRendered
)

// LogObject logs an entire object to the lane.
func LogObject(l Lane, level LaneLogLevel, message string, obj any) {
	li := l.(laneInternal)

	logObjectInternal(li.LaneProps(), li, level, message, obj)
}

func logObjectInternal(props loggingProperties, li laneInternal, level LaneLogLevel, message string, obj any) {
	// Convert the entire object (public and private values) to public
	o := CaptureObject(obj)

	raw, err := json.Marshal(&o)
	if err != nil {
		panic(err)
	}
	enc := fmt.Sprintf("%s: %s", message, string(raw))

	enc = li.Constrain(enc)

	switch level {
	case LogLevelTrace:
		li.TraceInternal(props, enc)
	case LogLevelDebug:
		li.DebugInternal(props, enc)
	case LogLevelInfo:
		li.InfoInternal(props, enc)
	case LogLevelWarn:
		li.WarnInternal(props, enc)
	case LogLevelError:
		li.ErrorInternal(props, enc)
	case logLevelPreFatal:
		li.PreFatalInternal(props, enc)
	case LogLevelFatal:
		li.FatalInternal(props, enc)
		li.OnPanic()
	default:
		panic("invalid level argument")
	}
}

func captureAddrs(val reflect.Value, addrs map[uintptr]recursionType) (showAddrs bool) {
	var addr uintptr
	if val.Kind() == reflect.Pointer {
		addr = val.Pointer()
	} else if val.Kind() == reflect.UnsafePointer {
		if val.CanAddr() {
			addr = val.UnsafeAddr()
		} else {
			addr = val.Pointer()
		}
	}

	if addr != 0 {
		n := addrs[addr]
		if n == recursionNone {
			addrs[addr] = recursionPossible
		} else {
			showAddrs = true
			if n == recursionPossible {
				addrs[addr] = recursionFirst
			}
			return
		}
	}

	switch val.Kind() {
	case reflect.Interface, reflect.Pointer:
		showAddrs = captureAddrs(val.Elem(), addrs) || showAddrs

	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			f := val.Field(i)
			showAddrs = captureAddrs(f, addrs) || showAddrs
		}

	case reflect.Array, reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			showAddrs = captureAddrs(val.Index(i), addrs) || showAddrs
		}

	case reflect.Map:
		iter := val.MapRange()
		for iter.Next() {
			rk := iter.Key()
			rv := iter.Value()
			showAddrs = captureAddrs(rk, addrs) || showAddrs
			showAddrs = captureAddrs(rv, addrs) || showAddrs
		}
	}

	return
}

func innerValue(val reflect.Value, addrs map[uintptr]recursionType) (inner any) {
	var pointerTarget uintptr
	if addrs != nil {
		if val.Kind() == reflect.Pointer {
			pointerTarget = val.Pointer()
		} else if val.Kind() == reflect.UnsafePointer {
			// Ensure the value is addressable before accessing UnsafeAddr
			if val.CanAddr() {
				pointerTarget = val.UnsafeAddr()
			} else {
				pointerTarget = val.Pointer()
			}
		}

		if pointerTarget != 0 {
			recursion := addrs[pointerTarget]
			if recursion == recursionWasRendered {
				return fmt.Sprintf("(pointer: %#x)", pointerTarget)
			} else if recursion == recursionFirst {
				addrs[pointerTarget] = recursionWasRendered
			} else {
				pointerTarget = 0
			}
		}
	}

	switch val.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.String:
		inner = val.Interface()

	case reflect.Float32, reflect.Float64:
		f64 := val.Float()
		if math.IsInf(f64, 0) {
			inner = fmt.Sprintf("%v", f64)
		} else {
			inner = val.Interface()
		}

	case reflect.Complex64:
		inner = fmt.Sprintf("%v", complex64(val.Complex()))

	case reflect.Complex128:
		inner = fmt.Sprintf("%v", val.Complex())

	case reflect.Chan:
		inner = fmt.Sprintf("%T", val.Interface())

	case reflect.Func:
		inner = runtime.FuncForPC(val.Pointer()).Name()

	case reflect.Struct:
		m := map[string]any{}
		val2 := reflect.New(val.Type()).Elem()
		val2.Set(val)
		for i := 0; i < val.NumField(); i++ {
			rf := val2.Field(i)
			rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
			m[val.Type().Field(i).Name] = innerValue(rf, addrs)
		}
		inner = m

	case reflect.Array, reflect.Slice:
		a := []any{}
		for i := 0; i < val.Len(); i++ {
			a = append(a, innerValue(val.Index(i), addrs))
		}

		// special case for byte array/slice: if the values are all ascii, render the bytes as runes
		if len(a) > 0 {
			if len(a) < 1000 {
				_, is := a[0].(byte)
				if is {
					seq := make(asciiSequence, 0, len(a))
					runeable := true
					for _, item := range a {
						by := item.(byte)
						if by < 32 {
							if by != '\n' && by != '\r' && by != '\t' {
								runeable = false
								break
							}
						} else if by == '"' || by == '\\' {
							seq = append(seq, '\\')
						} else if by > 126 {
							runeable = false
							break
						}
						seq = append(seq, by)
					}
					if runeable {
						inner = seq
						break
					}
				}
			} else {
				// large byte array - render as base64
				bytes := make([]byte, 0, len(a))
				_, is := a[0].(byte)
				if is {
					for _, item := range a {
						bytes = append(bytes, item.(byte))
					}
					inner = base64.StdEncoding.EncodeToString(bytes)
					break
				}
			}
		}

		inner = a

	case reflect.Map:
		// generalize map
		m := map[string]any{}

		iter := val.MapRange()
		for iter.Next() {
			rk := iter.Key()
			rv := iter.Value()
			m[fmt.Sprintf("%v", innerValue(rk, addrs))] = innerValue(rv, addrs)
		}
		inner = m

	case reflect.Interface, reflect.Pointer:
		inner = innerValue(val.Elem(), addrs)

	case reflect.UnsafePointer:
		inner = fmt.Sprintf("(unsafe.Pointer: %#x)", val.Pointer())

	case reflect.Invalid:
		// zero value
		break

	default:
		panic("can't process type combination " + val.Kind().String())
	}

	if pointerTarget != 0 {
		m, is := inner.(map[string]any)
		if is {
			m[""] = fmt.Sprintf("Address: %#x", pointerTarget)
		}
	}

	return
}

// CaptureObject converts an arbitrary object into a JSON-renderable object.
// Unlike standard json.Marshal, this function includes private fields and
// follows pointers to capture the full state of the object.
func CaptureObject(obj any) (v any) {
	addrs := map[uintptr]recursionType{}
	val := reflect.ValueOf(obj)
	if !captureAddrs(val, addrs) {
		addrs = nil
	}
	return innerValue(val, addrs)
}

func (seq asciiSequence) MarshalJSON() ([]byte, error) {
	var sb strings.Builder
	sb.WriteRune('"')
	for _, by := range seq {
		if by >= 32 {
			sb.WriteByte(by)
		} else if by == '\n' {
			sb.WriteString(`\n`)
		} else if by == '\r' {
			sb.WriteString(`\r`)
		} else if by == '\t' {
			sb.WriteString(`\t`)
		}
	}
	sb.WriteRune('"')
	return []byte(sb.String()), nil
}

func copyConfigToDerivation(dest, src Lane) {
	if !isNil(src) {
		for i := LogLevelTrace; i < logLevelMax; i++ {
			old := src.EnableStackTrace(i, false)
			src.EnableStackTrace(i, old)
			dest.EnableStackTrace(i, old)
		}

		oldMaxLen := src.SetLengthConstraint(0)
		src.SetLengthConstraint(oldMaxLen)
		dest.SetLengthConstraint(oldMaxLen)
	}
}

func isNil(i any) bool {
	if i == nil {
		return true // interface itself is nil
	}

	// use reflection to check if the underlying value is nil
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface, reflect.Func, reflect.Chan:
		return v.IsNil()
	default:
		return false
	}
}

func (props loggingProperties) getMessagePrefix(level string) string {
	id := trimLaneId(props.laneId)

	if props.journeyId != "" {
		return fmt.Sprintf("%s {%s:%s}", level, props.journeyId, id)
	} else {
		return fmt.Sprintf("%s {%s}", level, id)
	}
}

func trimLaneId(id string) string {
	if len(id) > 10 {
		id = id[len(id)-10:]
	}
	return id
}

func makeLaneId() string {
	return uuid.New().String()
}

func cleanStack(buf []byte, skipCallers int) (lines []string) {
	full := strings.Split(strings.TrimSpace(string(buf)), "\n")

	// the top line is a title of some kind like "goroutine 7 [running]", so skip that
	top := 1

	// next skip all of the go-lane implementation
	for top < len(full) {
		line := full[top]
		if !strings.Contains(line, "go-lane.(*") {
			break
		}
		top += 2
	}

	// skip past the unwanted callers as requested by the client
	top += skipCallers * 2
	if len(full) < top {
		return
	}

	// trim ending whitespace
	bottom := len(full)
	for bottom > top {
		if full[bottom-1] != "" {
			break
		}
		bottom--
	}

	lines = full[top:bottom]
	return
}
