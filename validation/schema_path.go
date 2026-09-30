package validation

import "strings"

// AJV escapes schema pointer components with encodeURIComponent; instance paths
// use only JSON Pointer escaping. The engine uses different URI escaping rules.
func schemaPath(location string) string {
	location = strings.TrimPrefix(location, rootResource)
	head, fragment, hasFragment := strings.Cut(location, "#")
	if !hasFragment {
		return location
	}
	if fragment == "" && head != "" {
		return head
	}
	parts := strings.Split(fragment, "/")
	for i := 0; i+1 < len(parts); i++ {
		switch parts[i] {
		case "properties", "patternProperties", "definitions", "$defs", "dependencies":
			i++
			parts[i] = encodeComponent(parts[i])
		}
	}
	return head + "#" + strings.Join(parts, "/")
}

func encodeComponent(value string) string {
	const hex = "0123456789ABCDEF"
	var result strings.Builder
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("-_.!~*'()", rune(ch)) {
			result.WriteByte(ch)
		} else {
			result.WriteByte('%')
			result.WriteByte(hex[ch>>4])
			result.WriteByte(hex[ch&15])
		}
	}
	return result.String()
}
