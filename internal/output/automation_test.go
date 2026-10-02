package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/lcorneliussen/md365/internal/apierr"
)

func TestProjectionPreservesSelectedFieldsAndCollectionMetadata(t *testing.T) {
	data := []map[string]any{
		{"id": "message-1", "subject": "One", "from": map[string]any{"address": "one@example.com"}},
		{"id": "message-2", "subject": "Two", "from": map[string]any{"address": "two@example.com"}},
	}
	var stdout bytes.Buffer
	total := 7
	writer := New(Options{Format: FormatJSON, Stdout: &stdout, Stderr: &stdout, Select: []string{"id", "from.address"}})
	if err := writer.OK(data, WithCollectionPage(true, "opaque-cursor", &total)); err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(response.Data)
	var projected []map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"id": "message-1", "from": map[string]any{"address": "one@example.com"}},
		{"id": "message-2", "from": map[string]any{"address": "two@example.com"}},
	}
	if !reflect.DeepEqual(projected, want) {
		t.Fatalf("projected = %#v, want %#v", projected, want)
	}
	if response.Meta["count"] != float64(2) || response.Meta["has_more"] != true || response.Meta["continuation"] != "opaque-cursor" || response.Meta["total"] != float64(7) {
		t.Fatalf("collection meta = %#v", response.Meta)
	}
}

func TestProjectionRejectsAbsentFields(t *testing.T) {
	writer := New(Options{Format: FormatJSON, Stdout: &bytes.Buffer{}, Select: []string{"invented"}})
	err := writer.OK([]map[string]any{{"id": "message-1"}})
	if apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("missing projection field = %v", err)
	}
}

func TestProjectionValidatesEmptyTypedCollection(t *testing.T) {
	type message struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
	}

	var stdout bytes.Buffer
	writer := New(Options{Format: FormatResultsOnly, Stdout: &stdout, Select: []string{"id"}})
	if err := writer.OK([]message{}); err != nil {
		t.Fatalf("declared selector on empty collection: %v", err)
	}
	if stdout.String() != "[]\n" {
		t.Fatalf("results-only output = %q", stdout.String())
	}

	writer = New(Options{Format: FormatResultsOnly, Stdout: &bytes.Buffer{}, Select: []string{"invented"}})
	err := writer.OK([]message{})
	if apierr.As(err).Code != apierr.CodeUsage {
		t.Fatalf("undeclared selector on empty collection = %v", err)
	}
}

func TestResultsOnlyAlwaysEmitsJSON(t *testing.T) {
	var stdout bytes.Buffer
	writer := New(Options{Format: FormatResultsOnly, Stdout: &stdout})
	if err := writer.OK("plain text"); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "\"plain text\"\n" {
		t.Fatalf("results-only string = %q", stdout.String())
	}
}

func TestFailEmptyUsesStableError(t *testing.T) {
	writer := New(Options{Format: FormatJSON, Stdout: &bytes.Buffer{}, FailEmpty: true})
	err := writer.OK([]string{})
	if apierr.As(err).Code != apierr.CodeEmpty || ExitCodeFor(err) != 11 {
		t.Fatalf("empty result = %v, exit %d", err, ExitCodeFor(err))
	}
}

func TestCollectionsAlwaysPublishCountAndHasMoreState(t *testing.T) {
	var stdout bytes.Buffer
	writer := New(Options{Format: FormatJSON, Stdout: &stdout})
	if err := writer.OK([]string{"one"}); err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Meta["count"] != float64(1) {
		t.Fatalf("count metadata = %#v", response.Meta)
	}
	if value, exists := response.Meta["has_more"]; !exists || value != nil {
		t.Fatalf("unknown has_more metadata = %#v", response.Meta)
	}
}
