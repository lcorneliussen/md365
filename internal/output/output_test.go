package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/lcorneliussen/md365/internal/apierr"
)

func TestPolicyDeniedExitCode(t *testing.T) {
	if got := ExitCodeFor(apierr.Policy("blocked")); got != 8 {
		t.Fatalf("policy denied exit code = %d, want 8", got)
	}
}

func TestStableExitCodes(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{apierr.Usage("usage"), 1},
		{apierr.Graph(404, "not found"), 2},
		{apierr.Graph(401, "auth"), 3},
		{apierr.Graph(403, "forbidden"), 4},
		{apierr.Graph(429, "rate limited"), 5},
		{apierr.Network("network", nil), 6},
		{apierr.Graph(400, "graph"), 7},
		{apierr.Policy("policy"), 8},
		{apierr.Graph(409, "conflict"), 9},
		{apierr.Graph(503, "retryable"), 10},
		{apierr.Empty("empty"), 11},
	}
	for _, test := range tests {
		if got := ExitCodeFor(test.err); got != test.want {
			t.Errorf("ExitCodeFor(%v) = %d, want %d", test.err, got, test.want)
		}
	}
}

func TestStructuredErrorIncludesGraphHTTPStatus(t *testing.T) {
	var stderr bytes.Buffer
	New(Options{Format: FormatJSON, Stderr: &stderr}).Err(apierr.Graph(429, "slow down"))
	var response ErrorResponse
	if err := json.Unmarshal(stderr.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != apierr.CodeRateLimit || response.HTTPStatus != 429 {
		t.Fatalf("error response = %#v", response)
	}
}
