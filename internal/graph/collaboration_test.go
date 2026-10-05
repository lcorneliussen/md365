package graph

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEscapeDrivePath(t *testing.T) {
	got := escapeDrivePath("/Shared Documents/Project #1/")
	want := "Shared%20Documents/Project%20%231"
	if got != want {
		t.Fatalf("escapeDrivePath() = %q, want %q", got, want)
	}
}

func TestNewDriveItemSearchRequest(t *testing.T) {
	request := newDriveItemSearchRequest("Jahresabschluss 2023", 25, 10)
	if len(request.Requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(request.Requests))
	}
	query := request.Requests[0]
	if query.Query.QueryString != "Jahresabschluss 2023" || query.From != 25 || query.Size != 10 {
		t.Fatalf("unexpected search query: %#v", query)
	}
	if len(query.EntityTypes) != 1 || query.EntityTypes[0] != "driveItem" {
		t.Fatalf("entityTypes = %#v", query.EntityTypes)
	}
}

func TestLoopSearchQueryConstrainsFileTypes(t *testing.T) {
	if got, want := loopSearchQuery(""), "(filetype:loop OR filetype:fluid)"; got != want {
		t.Fatalf("empty Loop query = %q, want %q", got, want)
	}
	if got, want := loopSearchQuery(" project plan "), "(project plan) AND (filetype:loop OR filetype:fluid)"; got != want {
		t.Fatalf("Loop query = %q, want %q", got, want)
	}
}

func TestLoopComponentNameFilter(t *testing.T) {
	for _, name := range []string{"Plan.loop", "Legacy.FLUID", " spaced.loop "} {
		if !isLoopComponentName(name) {
			t.Errorf("%q was not recognized as a Loop component", name)
		}
	}
	for _, name := range []string{"Plan.pdf", "loop", "Plan.loop.pdf"} {
		if isLoopComponentName(name) {
			t.Errorf("%q was incorrectly recognized as a Loop component", name)
		}
	}
}

func TestSearchLoopComponentsFiltersWhilePaging(t *testing.T) {
	var offsets []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request searchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		query := request.Requests[0]
		offsets = append(offsets, query.From)
		w.Header().Set("Content-Type", "application/json")
		if query.From == 0 {
			fmt.Fprint(w, `{"value":[{"hitsContainers":[{"moreResultsAvailable":true,"hits":[{"rank":1,"resource":{"id":"pdf-1","name":"Not Loop.pdf","size":1,"file":{"mimeType":"application/pdf"}}}]}]}]}`)
			return
		}
		fmt.Fprint(w, `{"value":[{"hitsContainers":[{"moreResultsAvailable":false,"hits":[{"rank":2,"resource":{"id":"loop-1","name":"Plan.loop","size":1,"file":{"mimeType":"application/octet-stream"}}}]}]}]}`)
	}))
	defer server.Close()

	hits, err := NewClient("token").searchDriveItems(server.URL, loopSearchQuery("plan"), 1, func(hit DriveItemSearchHit) bool {
		return isLoopComponentName(hit.Resource.Name)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Resource.ID != "loop-1" {
		t.Fatalf("hits = %#v", hits)
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 1 {
		t.Fatalf("offsets = %#v, want [0 1]", offsets)
	}
}

func TestParseDriveItemSearchResponse(t *testing.T) {
	data := []byte(`{
		"value": [{
			"hitsContainers": [{
				"moreResultsAvailable": true,
				"hits": [{
					"hitId": "hit-1",
					"rank": 1,
					"summary": "match",
					"resource": {
						"id": "item-1",
						"name": "Jahresabschluss 2023",
						"webUrl": "https://example.test/item",
						"folder": {"childCount": 2},
						"parentReference": {"driveId": "drive-1", "siteId": "site-1", "path": "/drive/root:/Datenraum"}
					}
				}]
			}]
		}]
	}`)

	hits, more, err := parseDriveItemSearchResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !more || len(hits) != 1 {
		t.Fatalf("hits = %d, more = %v", len(hits), more)
	}
	if hits[0].Resource.ID != "item-1" || hits[0].Resource.ParentReference.DriveID != "drive-1" {
		t.Fatalf("unexpected hit: %#v", hits[0])
	}

	if _, err := json.Marshal(hits); err != nil {
		t.Fatalf("search hits should remain JSON serializable: %v", err)
	}
}

func TestNewMessageSearchRequest(t *testing.T) {
	request := newMessageSearchRequest("subject:budget", 25, 10, true)
	query := request.Requests[0]
	if query.Query.QueryString != "subject:budget" || query.From != 25 || query.Size != 10 {
		t.Fatalf("unexpected message search query: %#v", query)
	}
	if !query.EnableTopResults {
		t.Fatal("top results should be enabled")
	}
	if len(query.EntityTypes) != 1 || query.EntityTypes[0] != "message" {
		t.Fatalf("entityTypes = %#v", query.EntityTypes)
	}
}

func TestParseMessageSearchResponse(t *testing.T) {
	data := []byte(`{"value":[{"hitsContainers":[{"moreResultsAvailable":true,"hits":[{"hitId":"message-1","rank":1,"summary":"matched attachment","resource":{"id":"message-1","subject":"Budget","receivedDateTime":"2026-10-01T10:00:00Z","from":{"emailAddress":{"name":"Ada","address":"ada@example.com"}}}}]}]}]}`)
	hits, more, err := parseMessageSearchResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !more || len(hits) != 1 {
		t.Fatalf("hits = %d, more = %v", len(hits), more)
	}
	if hits[0].Resource.ID != "message-1" || hits[0].Resource.Subject != "Budget" {
		t.Fatalf("unexpected hit: %#v", hits[0])
	}
}

func TestSearchMessagesPagesAndStopsOnEmptyPage(t *testing.T) {
	type observedRequest struct {
		From  int
		Size  int
		Query string
	}
	var observed []observedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/search/query" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var request searchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		query := request.Requests[0]
		observed = append(observed, observedRequest{From: query.From, Size: query.Size, Query: query.Query.QueryString})

		w.Header().Set("Content-Type", "application/json")
		if query.From == 0 {
			fmt.Fprint(w, `{"value":[{"hitsContainers":[{"moreResultsAvailable":true,"hits":[`)
			for i := 0; i < 25; i++ {
				if i > 0 {
					fmt.Fprint(w, ",")
				}
				fmt.Fprintf(w, `{"hitId":"message-%d","rank":%d,"resource":{"id":"message-%d"}}`, i, i+1, i)
			}
			fmt.Fprint(w, `]}]}]}`)
			return
		}
		fmt.Fprint(w, `{"value":[{"hitsContainers":[{"moreResultsAvailable":true,"hits":[]}]}]}`)
	}))
	defer server.Close()

	hits, err := NewClient("token").searchMessages(server.URL+"/search/query", `subject:"Q4 plan"`, 30, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 25 {
		t.Fatalf("hits = %d, want 25", len(hits))
	}
	want := []observedRequest{
		{From: 0, Size: 25, Query: `subject:"Q4 plan"`},
		{From: 25, Size: 5, Query: `subject:"Q4 plan"`},
	}
	if len(observed) != len(want) {
		t.Fatalf("requests = %#v, want %#v", observed, want)
	}
	for i := range want {
		if observed[i] != want[i] {
			t.Fatalf("request[%d] = %#v, want %#v", i, observed[i], want[i])
		}
	}
}

func TestAmbiguousSearchItem(t *testing.T) {
	folderLike := DriveItem{File: &FileFacet{MimeType: "application/octet-stream"}}
	if !ambiguousSearchItem(folderLike) {
		t.Fatal("zero-byte octet-stream search result should be hydrated")
	}
	file := DriveItem{Size: 42, File: &FileFacet{MimeType: "application/pdf"}}
	if ambiguousSearchItem(file) {
		t.Fatal("ordinary file should not be hydrated")
	}
}
