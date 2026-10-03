package graph

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestCalendarViewPaginationLimitsMatchingEventsAfterFiltering(t *testing.T) {
	all := []Event{{ID: "matching-1"}}
	page := []Event{{ID: "ignored"}, {ID: "matching-2"}, {ID: "matching-3"}}
	got, scanned, complete, err := collectCalendarPage(all, page, 2, 10, 0, func(event Event) bool {
		return event.ID != "ignored"
	})
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Fatal("page did not complete the bounded calendar query")
	}
	if len(got) != 2 || got[1].ID != "matching-2" || scanned != 2 {
		t.Fatalf("bounded events = %#v, want first two events", got)
	}
}

func TestCalendarViewFilteringFailsExplicitlyAtScanLimit(t *testing.T) {
	_, _, _, err := collectCalendarPage(nil, []Event{{ID: "ignored-1"}, {ID: "ignored-2"}}, 1, 1, 0, func(Event) bool { return false })
	if err == nil || !strings.Contains(err.Error(), "inspected more than 1 events") {
		t.Fatalf("error = %v, want explicit scan-limit error", err)
	}
}
