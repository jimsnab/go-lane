package lane

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// DiffObjects generates a display string describing the differences between [a] and [b],
// or an empty string if no differences exist.
func DiffObjects(a, b any) string {
	a2 := CaptureObject(a)
	b2 := CaptureObject(b)

	return diffObject(a2, b2)
}

func objToString(o any) string {
	raw, _ := json.Marshal(o)
	return string(raw)
}

func diffObject(a, b any) string {
	if a == nil {
		if b == nil {
			return ""
		} else {
			return fmt.Sprintf("[nil to %s]", objToString(b))
		}
	} else if b == nil {
		return fmt.Sprintf("[%s to nil]", objToString(a))
	}

	ta := fmt.Sprintf("%T", a)
	tb := fmt.Sprintf("%T", b)
	if ta != tb {
		return fmt.Sprintf("[type change %s to %s: was %s, is %s]", ta, tb, objToString(a), objToString(b))
	}

	// handle each json type
	switch aa := a.(type) {
	case map[string]any:
		bb := b.(map[string]any)
		return diffMap(aa, bb)

	case []any:
		bb := b.([]any)
		return diffArray(aa, bb)

	case float64:
		bb := b.(float64)
		if aa != bb {
			return fmt.Sprintf("[%f->%f]", aa, bb)
		}

	case int64:
		bb := b.(int64)
		if aa != bb {
			return fmt.Sprintf("[%d->%d]", aa, bb)
		}

	case string:
		bb := b.(string)
		if aa != bb {
			return fmt.Sprintf(`["%s"->"%s"]`, aa, bb)
		}

	case bool:
		bb := b.(bool)
		if aa != bb {
			return fmt.Sprintf("[%t->%t]", aa, bb)
		}
	}

	return ""
}

func mergeKeys[T any](a, b map[string]T) []string {
	keyTable := map[string]struct{}{}
	for k := range a {
		keyTable[k] = struct{}{}
	}
	for k := range b {
		keyTable[k] = struct{}{}
	}
	allKeys := make([]string, 0, len(keyTable))

	for k := range keyTable {
		allKeys = append(allKeys, k)
	}

	slices.Sort(allKeys)
	return allKeys
}

func diffMap(a, b map[string]any) string {
	allKeys := mergeKeys(a, b)

	var sb strings.Builder
	for _, k := range allKeys {
		aa := a[k]
		bb := b[k]

		if aa == nil {
			if bb != nil {
				sb.WriteString(fmt.Sprintf(`[new key "%s": %s]`, k, objToString(bb)))
			}
		} else if bb == nil {
			sb.WriteString(fmt.Sprintf(`[delete key "%s" was %s]`, k, objToString(aa)))
		} else {
			diff := diffObject(aa, bb)
			if diff != "" {
				sb.WriteString(fmt.Sprintf(`[%s: %s -> %s]`, k, objToString(aa), objToString(bb)))
			}
		}
	}

	return sb.String()
}

func diffArray(a, b []any) string {
	var sb strings.Builder
	i, j := 0, 0

	for i < len(a) && j < len(b) {
		if objToString(a[i]) == objToString(b[j]) {
			i++
			j++
		} else {
			if j+1 < len(b) && objToString(a[i]) == objToString(b[j+1]) {
				sb.WriteString(fmt.Sprintf(`[insert[%d]: %s]`, j, objToString(b[j])))
				j++
			} else if i+1 < len(a) && objToString(a[i+1]) == objToString(b[j]) {
				sb.WriteString(fmt.Sprintf(`[remove[%d]: %s]`, i, objToString(a[i])))
				i++
			} else {
				diff := diffObject(a[i], b[j])
				if diff == "" {
					diff = fmt.Sprintf("[%s->%s]", objToString(a[i]), objToString(b[j]))
				}
				sb.WriteString(fmt.Sprintf(`[replace[%d]: %s]`, j, diff))
				i++
				j++
			}
		}
	}

	for i < len(a) {
		sb.WriteString(fmt.Sprintf(`[remove[%d]: %s]`, i, objToString(a[i])))
		i++
	}

	for j < len(b) {
		sb.WriteString(fmt.Sprintf(`[append[%d]: %s]`, j, objToString(b[j])))
		j++
	}

	return sb.String()
}
