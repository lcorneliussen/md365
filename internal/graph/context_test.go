package graph

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCalendarViewURLNormalizesOffsetsToUTC(t *testing.T) {
	berlin := time.FixedZone("CEST", 2*60*60)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, berlin)
	end := time.Date(2026, 10, 1, 23, 59, 59, 0, berlin)

	parsed, err := url.Parse(calendarViewURL(start, end))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := parsed.Query().Get("startDateTime"), "2026-09-30T22:00:00Z"; got != want {
		t.Fatalf("startDateTime = %q, want %q", got, want)
	}
	if got, want := parsed.Query().Get("endDateTime"), "2026-10-01T21:59:59Z"; got != want {
		t.Fatalf("endDateTime = %q, want %q", got, want)
	}
}

func TestGraphRequestHonorsCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("canceled request reached fake Microsoft Graph")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewClientWithContext(ctx, "token")
	_, err := client.doRequest(http.MethodGet, server.URL, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestCalendarViewPaginationStopsAtLimit(t *testing.T) {
	all := []Event{{ID: "event-1"}}
	page := []Event{{ID: "event-2"}, {ID: "event-3"}}
	got, complete := appendEventPage(all, page, 2)
	if !complete {
		t.Fatal("page did not complete the bounded calendar query")
	}
	if len(got) != 2 || got[1].ID != "event-2" {
		t.Fatalf("bounded events = %#v, want first two events", got)
	}
}
