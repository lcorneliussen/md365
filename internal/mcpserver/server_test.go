package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lcorneliussen/md365/internal/cal"
	"github.com/lcorneliussen/md365/internal/mail"
	"github.com/lcorneliussen/md365/internal/storage"
	"github.com/lcorneliussen/md365/internal/teams"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDefinitionsAreFixedReadOnlyMicrosoft365Surface(t *testing.T) {
	definitions, err := Definitions()
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{
		"calendar_list", "channels_list", "drive_items_list", "files_search",
		"mail_get", "mail_search", "sharepoint_libraries", "teams_list",
	}
	gotNames := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		gotNames = append(gotNames, definition.Name)
		if len(definition.DelegatedPermissions) == 0 {
			t.Errorf("tool %s has no delegated permissions", definition.Name)
		}
		for _, permission := range definition.DelegatedPermissions {
			if strings.Contains(strings.ToLower(permission), "write") {
				t.Errorf("tool %s requests write permission %s", definition.Name, permission)
			}
		}
	}
	sort.Strings(gotNames)
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("tools = %v, want %v", gotNames, wantNames)
	}
}

func TestTypedToolsAgainstFakeMicrosoftGraph(t *testing.T) {
	graph := fakeMicrosoftGraph(t)
	defer graph.Close()

	server, err := New(Options{Backend: &fakeGraphBackend{baseURL: graph.URL}, Accounts: []string{"work"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	client := connectTestClient(t, ctx, server)

	listed, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 8 {
		t.Fatalf("listed %d tools, want 8", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Errorf("tool %s is missing a typed schema", tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
			t.Errorf("tool %s has unsafe annotations: %#v", tool.Name, tool.Annotations)
		}
	}

	calls := []struct {
		name     string
		args     map[string]any
		workload string
		resource string
	}{
		{"mail_search", map[string]any{"account": "work", "query": "quarterly results"}, "exchange_online", "message"},
		{"mail_get", map[string]any{"account": "work", "id": "message-1"}, "exchange_online", "message"},
		{"files_search", map[string]any{"account": "work", "query": "annual report"}, "onedrive_sharepoint", "drive_item"},
		{"sharepoint_libraries", map[string]any{"account": "work", "team_id": "team-1"}, "sharepoint", "document_library"},
		{"drive_items_list", map[string]any{"account": "work", "drive_id": "drive-1"}, "onedrive_sharepoint", "drive_item"},
		{"calendar_list", map[string]any{"account": "work", "from": "2026-10-01", "to": "2026-10-31"}, "exchange_online", "event"},
		{"teams_list", map[string]any{"account": "work"}, "microsoft_teams", "team"},
		{"channels_list", map[string]any{"account": "work", "team_id": "team-1"}, "microsoft_teams", "channel"},
	}
	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("tool returned error: %#v", result.Content)
			}
			payload := asMap(t, result.StructuredContent)
			wrapped, ok := payload["data"].(map[string]any)
			if !ok {
				data := payload["data"].([]any)
				if len(data) != 1 {
					t.Fatalf("data length = %d, want 1", len(data))
				}
				wrapped = data[0].(map[string]any)
			}
			if wrapped["untrusted"] != true {
				t.Fatalf("resource is not marked untrusted: %#v", wrapped)
			}
			source := wrapped["source"].(map[string]any)
			if source["workload"] != call.workload || source["resource"] != call.resource || source["account"] != "work" {
				t.Fatalf("unexpected provenance: %#v", source)
			}
		})
	}
}

func TestToolRejectsUnknownAccountWithStableJSONError(t *testing.T) {
	server, err := New(Options{Backend: &fakeGraphBackend{}, Accounts: []string{"work"}})
	if err != nil {
		t.Fatal(err)
	}
	client := connectTestClient(t, context.Background(), server)
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "teams_list", Arguments: map[string]any{"account": "other"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || len(result.Content) != 1 {
		t.Fatalf("expected MCP tool error, got %#v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("error content = %T, want text", result.Content[0])
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("error is not JSON: %q: %v", text.Text, err)
	}
	if payload["code"] != "usage" || payload["ok"] != false {
		t.Fatalf("unexpected error contract: %#v", payload)
	}
}

func TestCalendarRangeUsesConfiguredTimezoneAndPreservesRFC3339Offset(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	from, to, err := calendarRange("2026-10-01", "2026-10-01", berlin)
	if err != nil {
		t.Fatal(err)
	}
	if from.Location() != berlin || from.Hour() != 0 {
		t.Fatalf("date-only start = %s, want midnight Europe/Berlin", from)
	}
	if to.Location() != berlin || to.Hour() != 23 || to.Minute() != 59 {
		t.Fatalf("date-only end = %s, want end of day Europe/Berlin", to)
	}
	_, dstEnd, err := calendarRange("2026-10-25", "2026-10-25", berlin)
	if err != nil {
		t.Fatal(err)
	}
	if dstEnd.Day() != 25 || dstEnd.Hour() != 23 || dstEnd.Minute() != 59 {
		t.Fatalf("DST date-only end = %s, want local end of 2026-10-25", dstEnd)
	}

	offsetFrom, _, err := calendarRange("2026-10-01T00:00:00+02:00", "2026-10-02T00:00:00+02:00", berlin)
	if err != nil {
		t.Fatal(err)
	}
	_, offset := offsetFrom.Zone()
	if offset != 2*60*60 {
		t.Fatalf("RFC3339 offset = %d, want 7200", offset)
	}
}

func connectTestClient(t *testing.T, ctx context.Context, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "md365-test", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func asMap(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func fakeMicrosoftGraph(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, value any) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(value); err != nil {
			t.Error(err)
		}
	}
	mux.HandleFunc("/v1.0/search/messages", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []mail.SearchResultInfo{{MessageInfo: mail.MessageInfo{ID: "message-1", Account: "work", Subject: "Quarterly results"}, Rank: 1}})
	})
	mux.HandleFunc("/v1.0/search/driveItems", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []storage.SearchResultInfo{{ItemInfo: storage.ItemInfo{ID: "item-1", DriveID: "drive-1", Account: "work", Name: "Annual report.pdf", Type: "file"}, Rank: 1}})
	})
	mux.HandleFunc("/v1.0/me/calendarView", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []cal.EventInfo{{ID: "event-1", Account: "work", Subject: "Board review", Start: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)}})
	})
	mux.HandleFunc("/v1.0/me/joinedTeams", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []teams.TeamInfo{{ID: "team-1", Account: "work", DisplayName: "Finance"}})
	})
	return httptest.NewServer(mux)
}

type fakeGraphBackend struct {
	baseURL string
}

func (b *fakeGraphBackend) get(ctx context.Context, path string, output any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+path, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fake Microsoft Graph returned %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(output)
}

func (b *fakeGraphBackend) MailSearch(ctx context.Context, _, _ string, _ int, _ bool) ([]mail.SearchResultInfo, error) {
	var values []mail.SearchResultInfo
	return values, b.get(ctx, "/v1.0/search/messages", &values)
}

func (b *fakeGraphBackend) MailGet(context.Context, string, string) (*mail.MessageInfo, error) {
	return &mail.MessageInfo{ID: "message-1", Account: "work"}, nil
}

func (b *fakeGraphBackend) FilesSearch(ctx context.Context, _, _ string, _ int) ([]storage.SearchResultInfo, error) {
	var values []storage.SearchResultInfo
	return values, b.get(ctx, "/v1.0/search/driveItems", &values)
}

func (b *fakeGraphBackend) SharePointLibraries(context.Context, string, string, string, int) ([]storage.LibraryInfo, error) {
	return []storage.LibraryInfo{{ID: "drive-1", Account: "work", Name: "Documents"}}, nil
}

func (b *fakeGraphBackend) DriveItemsList(context.Context, string, string, string, string, int) ([]storage.ItemInfo, error) {
	return []storage.ItemInfo{{ID: "item-1", DriveID: "drive-1", Account: "work", Name: "Report.pdf"}}, nil
}

func (b *fakeGraphBackend) CalendarList(ctx context.Context, _ string, _, _ time.Time, _ string) ([]cal.EventInfo, error) {
	var values []cal.EventInfo
	return values, b.get(ctx, "/v1.0/me/calendarView", &values)
}

func (b *fakeGraphBackend) TeamsList(ctx context.Context, _ string, _ int) ([]teams.TeamInfo, error) {
	var values []teams.TeamInfo
	return values, b.get(ctx, "/v1.0/me/joinedTeams", &values)
}

func (b *fakeGraphBackend) ChannelsList(context.Context, string, string, int) ([]teams.ChannelInfo, error) {
	return []teams.ChannelInfo{{ID: "channel-1", TeamID: "team-1", Account: "work", DisplayName: "General"}}, nil
}
