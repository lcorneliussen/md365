package graph

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcorneliussen/md365/internal/apierr"
)

func TestResponseBodyReadFailuresAreClassified(t *testing.T) {
	for _, test := range []struct {
		status   int
		wantCode string
	}{
		{status: http.StatusOK, wantCode: apierr.CodeNetwork},
		{status: http.StatusServiceUnavailable, wantCode: apierr.CodeRetryable},
	} {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte("short"))
			}))
			defer server.Close()

			_, err := NewClient("token").doRequest(http.MethodGet, server.URL, nil)
			graphErr := apierr.As(err)
			if graphErr.Code != test.wantCode {
				t.Fatalf("code = %q, want %q (error: %v)", graphErr.Code, test.wantCode, err)
			}
			if test.status >= 400 && graphErr.HTTPStatus != test.status {
				t.Fatalf("HTTP status = %d, want %d", graphErr.HTTPStatus, test.status)
			}
		})
	}
}
