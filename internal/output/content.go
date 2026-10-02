package output

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

type ContentSafetyOptions struct {
	Wrap     bool
	Sanitize bool
}

type UntrustedSource struct {
	Workload   string `json:"workload"`
	Resource   string `json:"resource"`
	Account    string `json:"account,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	DriveID    string `json:"drive_id,omitempty"`
	TeamID     string `json:"team_id,omitempty"`
}

type UntrustedContent struct {
	Untrusted bool            `json:"untrusted"`
	Field     string          `json:"field"`
	Source    UntrustedSource `json:"source"`
	Content   any             `json:"content"`
}

// ProtectUntrusted transforms fields explicitly tagged as remote Microsoft 365
// content. Untagged IDs, timestamps, URLs, and command metadata retain their
// original JSON shape.
func ProtectUntrusted(data any, opts ContentSafetyOptions) any {
	if !opts.Wrap && !opts.Sanitize {
		return data
	}
	return protectValue(reflect.ValueOf(data), opts)
}

func protectValue(value reflect.Value, opts ContentSafetyOptions) any {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil
	}

	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		if !typeContainsUntrusted(value.Type().Elem(), map[reflect.Type]bool{}) {
			return value.Interface()
		}
		result := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = protectValue(value.Index(i), opts)
		}
		return result
	case reflect.Struct:
		if !typeContainsUntrusted(value.Type(), map[reflect.Type]bool{}) {
			return value.Interface()
		}
		return protectStruct(value, opts)
	default:
		return value.Interface()
	}
}

func protectStruct(value reflect.Value, opts ContentSafetyOptions) map[string]any {
	result := map[string]any{}
	typeOf := value.Type()
	for i := 0; i < value.NumField(); i++ {
		fieldType := typeOf.Field(i)
		if fieldType.PkgPath != "" {
			continue
		}
		fieldValue := value.Field(i)
		name, omitEmpty, skip := jsonField(fieldType)
		if skip {
			continue
		}
		if fieldType.Anonymous && name == "" {
			if nested, ok := protectValue(fieldValue, opts).(map[string]any); ok {
				for key, item := range nested {
					result[key] = item
				}
			}
			continue
		}
		if name == "" {
			name = fieldType.Name
		}
		if omitEmpty && fieldValue.IsZero() {
			continue
		}

		if tag := fieldType.Tag.Get("untrusted"); tag != "" {
			content := fieldValue.Interface()
			if opts.Sanitize {
				content = sanitizeValue(content)
			}
			if opts.Wrap {
				workload, resource := parseUntrustedTag(tag)
				result[name] = UntrustedContent{
					Untrusted: true,
					Field:     name,
					Source:    sourceFor(value, workload, resource),
					Content:   content,
				}
			} else {
				result[name] = content
			}
			continue
		}

		if typeContainsUntrusted(fieldValue.Type(), map[reflect.Type]bool{}) {
			result[name] = protectValue(fieldValue, opts)
		} else {
			result[name] = fieldValue.Interface()
		}
	}
	return result
}

func typeContainsUntrusted(value reflect.Type, seen map[reflect.Type]bool) bool {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct || seen[value] {
		return false
	}
	seen[value] = true
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.Tag.Get("untrusted") != "" {
			return true
		}
		if field.Anonymous && typeContainsUntrusted(field.Type, seen) {
			return true
		}
	}
	return false
}

func jsonField(field reflect.StructField) (name string, omitEmpty, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, part := range parts[1:] {
		if part == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func parseUntrustedTag(tag string) (string, string) {
	parts := strings.SplitN(tag, ",", 2)
	if len(parts) == 1 {
		return parts[0], "content"
	}
	return parts[0], parts[1]
}

func sourceFor(value reflect.Value, workload, resource string) UntrustedSource {
	return UntrustedSource{
		Workload: workload, Resource: resource,
		Account: stringField(value, "Account"), ResourceID: stringField(value, "ID"),
		DriveID: stringField(value, "DriveID"), TeamID: stringField(value, "TeamID"),
	}
}

func stringField(value reflect.Value, name string) string {
	field := value.FieldByName(name)
	for field.IsValid() && (field.Kind() == reflect.Pointer || field.Kind() == reflect.Interface) {
		if field.IsNil() {
			return ""
		}
		field = field.Elem()
	}
	if field.IsValid() && field.Kind() == reflect.String {
		return field.String()
	}
	return ""
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case string:
		return SanitizeContent(typed)
	case []string:
		result := make([]string, len(typed))
		for i, item := range typed {
			result[i] = SanitizeContent(item)
		}
		return result
	default:
		return value
	}
}

// SanitizeContent makes invisible/control characters explicit while retaining
// their code points. Newlines and tabs remain intact; CRLF is normalized.
func SanitizeContent(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	var result strings.Builder
	for _, r := range value {
		if r == '\n' || r == '\t' || (!unicode.Is(unicode.Cc, r) && !unicode.Is(unicode.Cf, r)) {
			result.WriteRune(r)
			continue
		}
		fmt.Fprintf(&result, `\u{%04X}`, r)
	}
	return result.String()
}
