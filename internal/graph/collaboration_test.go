package graph

import (
	"encoding/json"
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
