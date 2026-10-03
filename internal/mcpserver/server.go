package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lcorneliussen/md365/internal/apierr"
	"github.com/lcorneliussen/md365/internal/cal"
	"github.com/lcorneliussen/md365/internal/capability"
	"github.com/lcorneliussen/md365/internal/commandmeta"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/mail"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/lcorneliussen/md365/internal/storage"
	"github.com/lcorneliussen/md365/internal/teams"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const ServerVersion = "1.0.0"

type Options struct {
	Backend  Backend
	Accounts []string
	Timezone string
}

type Definition struct {
	Name                 string   `json:"name"`
	Command              string   `json:"command"`
	Description          string   `json:"description"`
	DelegatedPermissions []string `json:"delegated_permissions"`
}

var toolCommands = []struct {
	name    string
	command string
}{
	{"mail_search", "mail search"},
	{"mail_get", "mail get"},
	{"files_search", "files search"},
	{"sharepoint_libraries", "sharepoint libraries"},
	{"drive_items_list", "sharepoint list"},
	{"calendar_list", "cal list"},
	{"teams_list", "teams list"},
	{"channels_list", "teams channels"},
}

func Definitions() ([]Definition, error) {
	definitions := make([]Definition, 0, len(toolCommands))
	for _, entry := range toolCommands {
		policy, ok := commandmeta.Lookup(entry.command)
		if !ok {
			return nil, fmt.Errorf("MCP tool %s references unknown command %q", entry.name, entry.command)
		}
		if policy.Mutability != commandmeta.Read {
			return nil, fmt.Errorf("MCP tool %s references mutating command %q", entry.name, entry.command)
		}
		command, ok := capability.CommandByName(entry.command)
		if !ok {
			return nil, fmt.Errorf("MCP tool %s has no delegated-permission contract for %q", entry.name, entry.command)
		}
		scopes := append([]string(nil), command.Scopes...)
		for _, conditional := range command.ConditionalScopes {
			scopes = append(scopes, conditional.Scopes...)
		}
		scopes = capability.MinimalScopes(scopes)
		definitions = append(definitions, Definition{
			Name: entry.name, Command: entry.command, Description: command.Description,
			DelegatedPermissions: scopes,
		})
	}
	return definitions, nil
}

type serverState struct {
	backend  Backend
	accounts map[string]bool
	defs     map[string]Definition
	location *time.Location
}

func New(opts Options) (*mcp.Server, error) {
	if opts.Backend == nil {
		return nil, errors.New("MCP backend is required")
	}
	definitions, err := Definitions()
	if err != nil {
		return nil, err
	}
	location := time.UTC
	if opts.Timezone != "" {
		location, err = time.LoadLocation(opts.Timezone)
		if err != nil {
			return nil, fmt.Errorf("invalid md365 timezone %q: %w", opts.Timezone, err)
		}
	}
	state := &serverState{backend: opts.Backend, accounts: map[string]bool{}, defs: map[string]Definition{}, location: location}
	for _, account := range opts.Accounts {
		state.accounts[account] = true
	}
	for _, definition := range definitions {
		state.defs[definition.Name] = definition
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "md365", Version: ServerVersion}, nil)
	state.addTools(server)
	return server, nil
}

func NewProduction(cfg *config.Config) (*mcp.Server, error) {
	return New(Options{Backend: NewProductionBackend(cfg), Accounts: cfg.ListAccounts(), Timezone: cfg.Timezone})
}

type UntrustedResource[T any] struct {
	Untrusted bool                   `json:"untrusted"`
	Source    output.UntrustedSource `json:"source"`
	Content   T                      `json:"content"`
}

type CollectionMeta struct {
	Count   int  `json:"count"`
	Total   *int `json:"total,omitempty"`
	HasMore bool `json:"has_more"`
}

type CollectionResult[T any] struct {
	Data []UntrustedResource[T] `json:"data"`
	Meta CollectionMeta         `json:"meta"`
}

type ItemResult[T any] struct {
	Data UntrustedResource[T] `json:"data"`
}

type mailSearchInput struct {
	Account    string `json:"account" jsonschema:"configured md365 account name"`
	Query      string `json:"query" jsonschema:"Microsoft Search query for the signed-in Exchange Online mailbox"`
	Limit      int    `json:"limit,omitempty" jsonschema:"maximum messages to return; defaults to 25"`
	TopResults bool   `json:"top_results,omitempty" jsonschema:"promote Outlook top results before newest-first matches"`
}

type mailGetInput struct {
	Account string `json:"account" jsonschema:"configured md365 account name"`
	ID      string `json:"id" jsonschema:"stable Microsoft Graph message ID"`
}

type filesSearchInput struct {
	Account string `json:"account" jsonschema:"configured md365 account name"`
	Query   string `json:"query" jsonschema:"Microsoft Search query across visible OneDrive and SharePoint content"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum drive items to return; defaults to 25"`
}

type sharePointLibrariesInput struct {
	Account string `json:"account" jsonschema:"configured md365 account name"`
	TeamID  string `json:"team_id,omitempty" jsonschema:"Microsoft 365 group ID backing a Microsoft Teams team"`
	SiteID  string `json:"site_id,omitempty" jsonschema:"SharePoint site ID"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum document libraries to return; defaults to 100"`
}

type driveItemsInput struct {
	Account string `json:"account" jsonschema:"configured md365 account name"`
	DriveID string `json:"drive_id" jsonschema:"SharePoint or OneDrive document library drive ID"`
	ItemID  string `json:"item_id,omitempty" jsonschema:"folder DriveItem ID"`
	Path    string `json:"path,omitempty" jsonschema:"folder path relative to the drive root"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum child DriveItems to return; defaults to 100"`
}

type calendarListInput struct {
	Account string `json:"account" jsonschema:"configured md365 account name"`
	From    string `json:"from,omitempty" jsonschema:"range start as RFC3339 or YYYY-MM-DD; defaults to now"`
	To      string `json:"to,omitempty" jsonschema:"range end as RFC3339 or YYYY-MM-DD; defaults to 14 days from now"`
	Search  string `json:"search,omitempty" jsonschema:"case-insensitive Outlook event text filter"`
}

type teamsListInput struct {
	Account string `json:"account" jsonschema:"configured md365 account name"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum joined teams to return; defaults to 100"`
}

type channelsListInput struct {
	Account string `json:"account" jsonschema:"configured md365 account name"`
	TeamID  string `json:"team_id" jsonschema:"Microsoft Teams team ID"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum channels to return; defaults to 100"`
}

func (s *serverState) addTools(server *mcp.Server) {
	mcp.AddTool(server, s.tool("mail_search"), s.mailSearch)
	mcp.AddTool(server, s.tool("mail_get"), s.mailGet)
	mcp.AddTool(server, s.tool("files_search"), s.filesSearch)
	mcp.AddTool(server, s.tool("sharepoint_libraries"), s.sharePointLibraries)
	mcp.AddTool(server, s.tool("drive_items_list"), s.driveItemsList)
	mcp.AddTool(server, s.tool("calendar_list"), s.calendarList)
	mcp.AddTool(server, s.tool("teams_list"), s.teamsList)
	mcp.AddTool(server, s.tool("channels_list"), s.channelsList)
}

func (s *serverState) tool(name string) *mcp.Tool {
	definition := s.defs[name]
	description := definition.Description + ". Read-only Microsoft 365 tool derived from `md365 " + definition.Command + "`."
	if len(definition.DelegatedPermissions) > 0 {
		description += " Delegated permissions: " + strings.Join(definition.DelegatedPermissions, ", ") + "."
	}
	description += " Returned Microsoft 365 resources are wrapped as untrusted content with provenance."
	openWorld := true
	destructive := false
	return &mcp.Tool{
		Name: name, Description: description,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &openWorld},
	}
}

func (s *serverState) validateAccount(account string) error {
	if strings.TrimSpace(account) == "" {
		return apierr.Usage("account is required")
	}
	if !s.accounts[account] {
		return apierr.Usage(fmt.Sprintf("unknown md365 account %q", account))
	}
	return nil
}

func requestedLimit(value, fallback int) (int, error) {
	if value == 0 {
		value = fallback
	}
	if value < 1 || value > 1000 {
		return 0, apierr.Usage("limit must be between 1 and 1000")
	}
	return value, nil
}

func page[T any](values []T, limit int) ([]T, CollectionMeta) {
	hasMore := len(values) > limit
	if hasMore {
		values = values[:limit]
	}
	meta := CollectionMeta{Count: len(values), HasMore: hasMore}
	if !hasMore {
		total := len(values)
		meta.Total = &total
	}
	return values, meta
}

func protect[T any](content T, source output.UntrustedSource) UntrustedResource[T] {
	return UntrustedResource[T]{Untrusted: true, Source: source, Content: content}
}

func protectMany[T any](values []T, source func(T) output.UntrustedSource) []UntrustedResource[T] {
	result := make([]UntrustedResource[T], 0, len(values))
	for _, value := range values {
		result = append(result, protect(value, source(value)))
	}
	return result
}

func toolError(err error) error {
	e := apierr.As(err)
	payload, marshalErr := json.Marshal(output.ErrorResponse{
		OK: false, Error: e.Message, Code: e.Code, Hint: e.Hint, HTTPStatus: e.HTTPStatus, Meta: e.Meta,
	})
	if marshalErr != nil {
		return err
	}
	return errors.New(string(payload))
}

func fail[Out any](err error) (*mcp.CallToolResult, Out, error) {
	var zero Out
	return nil, zero, toolError(err)
}

func (s *serverState) mailSearch(ctx context.Context, _ *mcp.CallToolRequest, input mailSearchInput) (*mcp.CallToolResult, CollectionResult[mail.SearchResultInfo], error) {
	if err := s.validateAccount(input.Account); err != nil {
		return fail[CollectionResult[mail.SearchResultInfo]](err)
	}
	if strings.TrimSpace(input.Query) == "" {
		return fail[CollectionResult[mail.SearchResultInfo]](apierr.Usage("query is required"))
	}
	limit, err := requestedLimit(input.Limit, 25)
	if err != nil {
		return fail[CollectionResult[mail.SearchResultInfo]](err)
	}
	values, err := s.backend.MailSearch(ctx, input.Account, strings.TrimSpace(input.Query), limit+1, input.TopResults)
	if err != nil {
		return fail[CollectionResult[mail.SearchResultInfo]](err)
	}
	values, meta := page(values, limit)
	data := protectMany(values, func(value mail.SearchResultInfo) output.UntrustedSource {
		return output.UntrustedSource{Workload: "exchange_online", Resource: "message", Account: input.Account, ResourceID: value.ID}
	})
	return nil, CollectionResult[mail.SearchResultInfo]{Data: data, Meta: meta}, nil
}

func (s *serverState) mailGet(ctx context.Context, _ *mcp.CallToolRequest, input mailGetInput) (*mcp.CallToolResult, ItemResult[mail.MessageInfo], error) {
	if err := s.validateAccount(input.Account); err != nil {
		return fail[ItemResult[mail.MessageInfo]](err)
	}
	if strings.TrimSpace(input.ID) == "" {
		return fail[ItemResult[mail.MessageInfo]](apierr.Usage("id is required"))
	}
	value, err := s.backend.MailGet(ctx, input.Account, input.ID)
	if err != nil {
		return fail[ItemResult[mail.MessageInfo]](err)
	}
	source := output.UntrustedSource{Workload: "exchange_online", Resource: "message", Account: input.Account, ResourceID: value.ID}
	return nil, ItemResult[mail.MessageInfo]{Data: protect(*value, source)}, nil
}

func (s *serverState) filesSearch(ctx context.Context, _ *mcp.CallToolRequest, input filesSearchInput) (*mcp.CallToolResult, CollectionResult[storage.SearchResultInfo], error) {
	if err := s.validateAccount(input.Account); err != nil {
		return fail[CollectionResult[storage.SearchResultInfo]](err)
	}
	if strings.TrimSpace(input.Query) == "" {
		return fail[CollectionResult[storage.SearchResultInfo]](apierr.Usage("query is required"))
	}
	limit, err := requestedLimit(input.Limit, 25)
	if err != nil {
		return fail[CollectionResult[storage.SearchResultInfo]](err)
	}
	values, err := s.backend.FilesSearch(ctx, input.Account, strings.TrimSpace(input.Query), limit+1)
	if err != nil {
		return fail[CollectionResult[storage.SearchResultInfo]](err)
	}
	values, meta := page(values, limit)
	data := protectMany(values, func(value storage.SearchResultInfo) output.UntrustedSource {
		return output.UntrustedSource{Workload: "onedrive_sharepoint", Resource: "drive_item", Account: input.Account, ResourceID: value.ID, DriveID: value.DriveID}
	})
	return nil, CollectionResult[storage.SearchResultInfo]{Data: data, Meta: meta}, nil
}

func (s *serverState) sharePointLibraries(ctx context.Context, _ *mcp.CallToolRequest, input sharePointLibrariesInput) (*mcp.CallToolResult, CollectionResult[storage.LibraryInfo], error) {
	if err := s.validateAccount(input.Account); err != nil {
		return fail[CollectionResult[storage.LibraryInfo]](err)
	}
	if (input.TeamID == "") == (input.SiteID == "") {
		return fail[CollectionResult[storage.LibraryInfo]](apierr.Usage("choose exactly one of team_id or site_id"))
	}
	limit, err := requestedLimit(input.Limit, 100)
	if err != nil {
		return fail[CollectionResult[storage.LibraryInfo]](err)
	}
	values, err := s.backend.SharePointLibraries(ctx, input.Account, input.TeamID, input.SiteID, limit+1)
	if err != nil {
		return fail[CollectionResult[storage.LibraryInfo]](err)
	}
	values, meta := page(values, limit)
	data := protectMany(values, func(value storage.LibraryInfo) output.UntrustedSource {
		return output.UntrustedSource{Workload: "sharepoint", Resource: "document_library", Account: input.Account, DriveID: value.ID}
	})
	return nil, CollectionResult[storage.LibraryInfo]{Data: data, Meta: meta}, nil
}

func (s *serverState) driveItemsList(ctx context.Context, _ *mcp.CallToolRequest, input driveItemsInput) (*mcp.CallToolResult, CollectionResult[storage.ItemInfo], error) {
	if err := s.validateAccount(input.Account); err != nil {
		return fail[CollectionResult[storage.ItemInfo]](err)
	}
	if strings.TrimSpace(input.DriveID) == "" {
		return fail[CollectionResult[storage.ItemInfo]](apierr.Usage("drive_id is required"))
	}
	if input.ItemID != "" && input.Path != "" {
		return fail[CollectionResult[storage.ItemInfo]](apierr.Usage("item_id and path are mutually exclusive"))
	}
	limit, err := requestedLimit(input.Limit, 100)
	if err != nil {
		return fail[CollectionResult[storage.ItemInfo]](err)
	}
	values, err := s.backend.DriveItemsList(ctx, input.Account, input.DriveID, input.ItemID, input.Path, limit+1)
	if err != nil {
		return fail[CollectionResult[storage.ItemInfo]](err)
	}
	values, meta := page(values, limit)
	data := protectMany(values, func(value storage.ItemInfo) output.UntrustedSource {
		return output.UntrustedSource{Workload: "onedrive_sharepoint", Resource: "drive_item", Account: input.Account, ResourceID: value.ID, DriveID: value.DriveID}
	})
	return nil, CollectionResult[storage.ItemInfo]{Data: data, Meta: meta}, nil
}

func (s *serverState) calendarList(ctx context.Context, _ *mcp.CallToolRequest, input calendarListInput) (*mcp.CallToolResult, CollectionResult[cal.EventInfo], error) {
	if err := s.validateAccount(input.Account); err != nil {
		return fail[CollectionResult[cal.EventInfo]](err)
	}
	from, to, err := calendarRange(input.From, input.To, s.location)
	if err != nil {
		return fail[CollectionResult[cal.EventInfo]](err)
	}
	values, err := s.backend.CalendarList(ctx, input.Account, from, to, input.Search)
	if err != nil {
		return fail[CollectionResult[cal.EventInfo]](err)
	}
	total := len(values)
	data := protectMany(values, func(value cal.EventInfo) output.UntrustedSource {
		return output.UntrustedSource{Workload: "exchange_online", Resource: "event", Account: input.Account, ResourceID: value.ID}
	})
	return nil, CollectionResult[cal.EventInfo]{Data: data, Meta: CollectionMeta{Count: total, Total: &total}}, nil
}

func (s *serverState) teamsList(ctx context.Context, _ *mcp.CallToolRequest, input teamsListInput) (*mcp.CallToolResult, CollectionResult[teams.TeamInfo], error) {
	if err := s.validateAccount(input.Account); err != nil {
		return fail[CollectionResult[teams.TeamInfo]](err)
	}
	limit, err := requestedLimit(input.Limit, 100)
	if err != nil {
		return fail[CollectionResult[teams.TeamInfo]](err)
	}
	values, err := s.backend.TeamsList(ctx, input.Account, limit+1)
	if err != nil {
		return fail[CollectionResult[teams.TeamInfo]](err)
	}
	values, meta := page(values, limit)
	data := protectMany(values, func(value teams.TeamInfo) output.UntrustedSource {
		return output.UntrustedSource{Workload: "microsoft_teams", Resource: "team", Account: input.Account, ResourceID: value.ID}
	})
	return nil, CollectionResult[teams.TeamInfo]{Data: data, Meta: meta}, nil
}

func (s *serverState) channelsList(ctx context.Context, _ *mcp.CallToolRequest, input channelsListInput) (*mcp.CallToolResult, CollectionResult[teams.ChannelInfo], error) {
	if err := s.validateAccount(input.Account); err != nil {
		return fail[CollectionResult[teams.ChannelInfo]](err)
	}
	if strings.TrimSpace(input.TeamID) == "" {
		return fail[CollectionResult[teams.ChannelInfo]](apierr.Usage("team_id is required"))
	}
	limit, err := requestedLimit(input.Limit, 100)
	if err != nil {
		return fail[CollectionResult[teams.ChannelInfo]](err)
	}
	values, err := s.backend.ChannelsList(ctx, input.Account, input.TeamID, limit+1)
	if err != nil {
		return fail[CollectionResult[teams.ChannelInfo]](err)
	}
	values, meta := page(values, limit)
	data := protectMany(values, func(value teams.ChannelInfo) output.UntrustedSource {
		return output.UntrustedSource{Workload: "microsoft_teams", Resource: "channel", Account: input.Account, ResourceID: value.ID, TeamID: value.TeamID}
	})
	return nil, CollectionResult[teams.ChannelInfo]{Data: data, Meta: meta}, nil
}

func calendarRange(fromValue, toValue string, location *time.Location) (time.Time, time.Time, error) {
	if location == nil {
		location = time.UTC
	}
	now := time.Now().In(location)
	from := now
	to := now.AddDate(0, 0, 14)
	var err error
	if fromValue != "" {
		from, err = parseCalendarTime(fromValue, false, location)
		if err != nil {
			return time.Time{}, time.Time{}, apierr.Usage("invalid from: " + err.Error())
		}
	}
	if toValue != "" {
		to, err = parseCalendarTime(toValue, true, location)
		if err != nil {
			return time.Time{}, time.Time{}, apierr.Usage("invalid to: " + err.Error())
		}
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, apierr.Usage("to must be after from")
	}
	return from, to, nil
}

func parseCalendarTime(value string, endOfDay bool, location *time.Location) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD")
	}
	if endOfDay {
		parsed = parsed.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
	return parsed, nil
}

func ToolNames() ([]string, error) {
	definitions, err := Definitions()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	sort.Strings(names)
	return names, nil
}
