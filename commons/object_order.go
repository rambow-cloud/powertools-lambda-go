package commons

import (
	"sort"
	"strconv"
)

// SortObjectKeys orders keys in place like JavaScript Object.keys: canonical array
// indices first in numeric order, followed by other keys in their existing order.
// Callers supply unique keys and preserve insertion order before calling it.
func SortObjectKeys(keys []string) {
	index := func(key string) (uint64, bool) {
		value, err := strconv.ParseUint(key, 10, 32)
		return value, err == nil && value < 1<<32-1 && strconv.FormatUint(value, 10) == key
	}
	sort.SliceStable(keys, func(i, j int) bool {
		a, ai := index(keys[i])
		b, bi := index(keys[j])
		return ai && (!bi || a < b)
	})
}
