package output

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/lcorneliussen/md365/internal/apierr"
)

func WithCollectionPage(hasMore bool, continuation string, total *int) ResponseOption {
	return func(response *Response) {
		if response.Meta == nil {
			response.Meta = map[string]any{}
		}
		response.Meta["has_more"] = hasMore
		if continuation != "" {
			response.Meta["continuation"] = continuation
		}
		if total != nil {
			response.Meta["total"] = *total
		}
	}
}

func isEmptyResult(data any) bool {
	if data == nil {
		return true
	}
	value := reflect.ValueOf(data)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return true
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return value.Len() == 0
	}
	return false
}

func collectionCount(data any) (int, bool) {
	if data == nil {
		return 0, false
	}
	value := reflect.ValueOf(data)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return 0, false
		}
		value = value.Elem()
	}
	if value.IsValid() && (value.Kind() == reflect.Slice || value.Kind() == reflect.Array) {
		return value.Len(), true
	}
	return 0, false
}

func projectFields(data any, fields []string) (any, error) {
	var normalized any
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare projection: %w", err)
	}
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, fmt.Errorf("failed to prepare projection: %w", err)
	}

	switch value := normalized.(type) {
	case []any:
		result := make([]any, len(value))
		for i, item := range value {
			object, ok := item.(map[string]any)
			if !ok {
				return nil, apierr.Usage("--select requires object results")
			}
			projected, err := projectObject(object, fields)
			if err != nil {
				return nil, err
			}
			result[i] = projected
		}
		return result, nil
	case map[string]any:
		return projectObject(value, fields)
	default:
		return nil, apierr.Usage("--select requires object or collection results")
	}
}

func projectObject(value map[string]any, fields []string) (map[string]any, error) {
	result := map[string]any{}
	for _, field := range fields {
		path := strings.Split(field, ".")
		selected, ok := getPath(value, path)
		if !ok {
			return nil, apierr.Usage(fmt.Sprintf("--select field %q is not present in the response", field))
		}
		setPath(result, path, selected)
	}
	return result, nil
}

func getPath(value map[string]any, path []string) (any, bool) {
	var current any = value
	for _, segment := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func setPath(value map[string]any, path []string, selected any) {
	current := value
	for _, segment := range path[:len(path)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	current[path[len(path)-1]] = selected
}
