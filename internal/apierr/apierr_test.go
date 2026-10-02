package apierr

import "testing"

func TestGraphClassifiesStableAutomationErrors(t *testing.T) {
	tests := map[int]string{
		400: CodeGraph, 401: CodeAuth, 403: CodeForbidden, 404: CodeNotFound,
		409: CodeConflict, 412: CodeConflict, 429: CodeRateLimit,
		500: CodeRetryable, 502: CodeRetryable, 503: CodeRetryable, 504: CodeRetryable,
	}
	for status, want := range tests {
		if got := Graph(status, "failed").Code; got != want {
			t.Fatalf("HTTP %d code = %q, want %q", status, got, want)
		}
	}
}
