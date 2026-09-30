package commons

import "reflect"

type mergeVisit struct {
	kind    reflect.Kind
	pointer uintptr
}

func object(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	v := reflect.ValueOf(value)
	t := reflect.TypeFor[map[string]any]()
	if v.Type().ConvertibleTo(t) {
		return v.Convert(t).Interface().(map[string]any), true
	}
	return nil, false
}
func array(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice || v.Type().Elem().Kind() == reflect.Uint8 {
		return nil, false
	}
	items := make([]any, v.Len())
	for i := range items {
		items[i] = v.Index(i).Interface()
	}
	return items, true
}
func identity(value any) mergeVisit {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Map {
		return mergeVisit{v.Kind(), uintptr(v.UnsafePointer())}
	}
	return mergeVisit{v.Kind(), v.Pointer()}
}

// DeepMerge mutates target, merges arrays by index, and clones incoming containers.
// Constructor/prototype keys and references to an ancestor are skipped like the reference.
// Go nil represents null and replaces an existing value; it is not JavaScript undefined.
func DeepMerge(target map[string]any, sources ...map[string]any) map[string]any {
	if target == nil {
		target = make(map[string]any)
	}
	ancestors := map[mergeVisit]bool{identity(target): true}
	for _, source := range sources {
		if source == nil || identity(source) == identity(target) {
			continue
		}
		ancestors[identity(source)] = true
		mergeMap(target, source, ancestors)
		delete(ancestors, identity(source))
	}
	return target
}
func mergeMap(target, source map[string]any, ancestors map[mergeVisit]bool) {
	for key, value := range source {
		if key == "__proto__" || key == "constructor" {
			continue
		}
		if merged, keep := mergeValue(target[key], value, ancestors); keep {
			target[key] = merged
		}
	}
}
func mergeValue(target, source any, ancestors map[mergeVisit]bool) (any, bool) {
	if src, ok := object(source); ok {
		id := identity(source)
		if ancestors[id] {
			return nil, false
		}
		ancestors[id] = true
		defer delete(ancestors, id)
		dst, ok := object(target)
		if !ok || dst == nil {
			dst = make(map[string]any)
		}
		mergeMap(dst, src, ancestors)
		return dst, true
	}
	if src, ok := array(source); ok {
		id := identity(source)
		if ancestors[id] {
			return nil, false
		}
		ancestors[id] = true
		defer delete(ancestors, id)
		dst, _ := array(target)
		for i, value := range src {
			var prior any
			if i < len(dst) {
				prior = dst[i]
			}
			merged, keep := mergeValue(prior, value, ancestors)
			if !keep {
				continue
			}
			for len(dst) <= i {
				dst = append(dst, nil)
			}
			dst[i] = merged
		}
		if dst == nil {
			dst = []any{}
		}
		return dst, true
	}
	return source, true
}
