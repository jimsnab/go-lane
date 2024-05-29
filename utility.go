package lane

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"unsafe"
)

func LogObject(l Lane, level LaneLogLevel, message string, obj any) {
	// Convert the entire object (public and private values) to public
	o := captureObject(obj)

	raw, err := json.Marshal(&o)
	if err != nil {
		panic(err)
	}
	enc := fmt.Sprintf("%s: %s", message, string(raw))

	switch level {
	case LogLevelTrace:
		l.Trace(enc)
	case LogLevelDebug:
		l.Debug(enc)
	case LogLevelInfo:
		l.Info(enc)
	case LogLevelWarn:
		l.Warn(enc)
	case LogLevelError:
		l.Error(enc)
	case logLevelPreFatal:
		l.PreFatal(enc)
	case LogLevelFatal:
		l.Fatal(enc)
	default:
		panic("invalid level argument")
	}
}

func innerValue(val reflect.Value) any {
	switch val.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String,
		reflect.Interface, reflect.Slice, reflect.Array, reflect.Chan, reflect.Func:
		return val.Interface()
	case reflect.Struct:
		// convert to a map
		m := map[string]any{}

		val2 := reflect.New(val.Type()).Elem()
		val2.Set(val)

		for i := 0; i < val.NumField(); i++ {
			rf := val2.Field(i)
			rf = reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()

			m[val.Type().Field(i).Name] = innerValue(rf)
		}
		return m
	case reflect.Map:
		// generalize map
		m := map[string]any{}

		iter := val.MapRange()
		for iter.Next() {
			rk := iter.Key()
			rv := iter.Value()
			m[fmt.Sprintf("%v", captureObject(innerValue(rk)))] = captureObject(innerValue(rv))
		}
		return m
	case reflect.Pointer:
		return innerValue(val.Elem())
	case reflect.Invalid:
		// zero value
		return nil
	}

	panic("can't process type combination")
}

func captureObject(obj any) (v any) {
	r := reflect.ValueOf(obj)
	switch r.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.String:
		v = obj

	case reflect.Complex64, reflect.Complex128:
		v = fmt.Sprintf("%v", obj)

	case reflect.Chan:
		v = fmt.Sprintf("%T", obj)

	case reflect.Func:
		v = runtime.FuncForPC(r.Pointer()).Name()

	case reflect.Interface, reflect.Pointer:
		v = captureObject(innerValue(r))

	case reflect.Array, reflect.Slice:
		a := []any{}

		for i := 0; i < r.Len(); i++ {
			a = append(a, captureObject(innerValue(r.Index(i))))
		}

		v = a

	case reflect.Map, reflect.Struct:
		v = innerValue(r)
	}

	return
}
