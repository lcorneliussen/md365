package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/lcorneliussen/md365/internal/apierr"
)

type Format int

const (
	FormatHuman Format = iota
	FormatJSON
	FormatQuiet
	FormatIDs
	FormatCount
)

type Breadcrumb struct {
	Action      string `json:"action"`
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

type Response struct {
	OK          bool           `json:"ok"`
	Data        any            `json:"data,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
	Breadcrumbs []Breadcrumb   `json:"breadcrumbs,omitempty"`
}

type ErrorResponse struct {
	OK    bool           `json:"ok"`
	Error string         `json:"error"`
	Code  string         `json:"code"`
	Hint  string         `json:"hint,omitempty"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type Options struct {
	Format Format
	Stdout io.Writer
	Stderr io.Writer
}

type ResponseOption func(*Response)

func WithSummary(summary string) ResponseOption {
	return func(r *Response) { r.Summary = summary }
}

func WithMeta(key string, value any) ResponseOption {
	return func(r *Response) {
		if r.Meta == nil {
			r.Meta = map[string]any{}
		}
		r.Meta[key] = value
	}
}

func WithBreadcrumbs(breadcrumbs ...Breadcrumb) ResponseOption {
	return func(r *Response) { r.Breadcrumbs = breadcrumbs }
}

type Writer struct {
	opts Options
}

func New(opts Options) *Writer {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return &Writer{opts: opts}
}

func (w *Writer) Format() Format {
	return w.opts.Format
}

func (w *Writer) IsHuman() bool {
	return w.opts.Format == FormatHuman
}

func (w *Writer) OK(data any, opts ...ResponseOption) error {
	resp := Response{OK: true, Data: normalizeData(data)}
	for _, opt := range opts {
		opt(&resp)
	}

	switch w.opts.Format {
	case FormatJSON:
		return writeJSON(w.opts.Stdout, resp)
	case FormatQuiet:
		return writeQuiet(w.opts.Stdout, resp.Data)
	case FormatIDs:
		return writeIDs(w.opts.Stdout, resp.Data)
	case FormatCount:
		return writeCount(w.opts.Stdout, resp.Data)
	default:
		if resp.Summary != "" {
			_, err := fmt.Fprintln(w.opts.Stdout, resp.Summary)
			return err
		}
		return writeQuiet(w.opts.Stdout, resp.Data)
	}
}

func (w *Writer) Err(err error) {
	e := apierr.As(err)
	if w.opts.Format == FormatJSON || w.opts.Format == FormatQuiet || w.opts.Format == FormatIDs || w.opts.Format == FormatCount {
		_ = writeJSON(w.opts.Stderr, ErrorResponse{
			OK:    false,
			Error: e.Message,
			Code:  e.Code,
			Hint:  e.Hint,
			Meta:  e.Meta,
		})
		return
	}

	fmt.Fprintf(w.opts.Stderr, "Error: %s\n", e.Message)
	if e.Hint != "" {
		fmt.Fprintln(w.opts.Stderr, e.Hint)
	}
}

func ExitCodeFor(err error) int {
	e := apierr.As(err)
	switch e.Code {
	case apierr.CodeUsage:
		return 1
	case apierr.CodeNotFound:
		return 2
	case apierr.CodeAuth:
		return 3
	case apierr.CodeForbidden:
		return 4
	case apierr.CodeRateLimit:
		return 5
	case apierr.CodeNetwork:
		return 6
	case apierr.CodeGraph:
		return 7
	default:
		return 1
	}
}

func normalizeData(data any) any {
	if data == nil {
		return nil
	}
	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Slice && value.IsNil() {
		return reflect.MakeSlice(value.Type(), 0, 0).Interface()
	}
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return nil
	}
	return data
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(value)
}

func writeQuiet(w io.Writer, data any) error {
	switch v := data.(type) {
	case nil:
		return nil
	case string:
		_, err := fmt.Fprintln(w, v)
		return err
	case []string:
		for _, item := range v {
			if _, err := fmt.Fprintln(w, item); err != nil {
				return err
			}
		}
		return nil
	default:
		return writeJSON(w, data)
	}
}

func writeIDs(w io.Writer, data any) error {
	ids, err := extractIDs(data)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := fmt.Fprintln(w, id); err != nil {
			return err
		}
	}
	return nil
}

func writeCount(w io.Writer, data any) error {
	count := 0
	value := reflect.ValueOf(data)
	if value.IsValid() {
		switch value.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
			count = value.Len()
		default:
			count = 1
		}
	}
	_, err := fmt.Fprintln(w, count)
	return err
}

func extractIDs(data any) ([]string, error) {
	value := reflect.ValueOf(data)
	if !value.IsValid() {
		return nil, nil
	}
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		id := idFromValue(value)
		if id == "" {
			return nil, apierr.Usage("--ids-only requires data with id fields")
		}
		return []string{id}, nil
	}

	ids := make([]string, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		id := idFromValue(value.Index(i))
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func idFromValue(value reflect.Value) string {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Map {
		for _, key := range []string{"id", "ID"} {
			mv := value.MapIndex(reflect.ValueOf(key))
			if mv.IsValid() {
				return strings.TrimSpace(fmt.Sprint(mv.Interface()))
			}
		}
		return ""
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	for _, name := range []string{"ID", "Id"} {
		field := value.FieldByName(name)
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	}
	return ""
}
