package mcpserver

import (
	"context"
	"time"

	"github.com/lcorneliussen/md365/internal/cal"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/mail"
	"github.com/lcorneliussen/md365/internal/storage"
	"github.com/lcorneliussen/md365/internal/teams"
)

// Backend is the fixed read-only Microsoft 365 surface used by the MCP server.
// It deliberately has no generic Microsoft Graph request or mutation method.
type Backend interface {
	MailSearch(context.Context, string, string, int, bool) ([]mail.SearchResultInfo, error)
	MailGet(context.Context, string, string) (*mail.MessageInfo, error)
	FilesSearch(context.Context, string, string, int) ([]storage.SearchResultInfo, error)
	SharePointLibraries(context.Context, string, string, string, int) ([]storage.LibraryInfo, error)
	DriveItemsList(context.Context, string, string, string, string, int) ([]storage.ItemInfo, error)
	CalendarList(context.Context, string, time.Time, time.Time, string) ([]cal.EventInfo, error)
	TeamsList(context.Context, string, int) ([]teams.TeamInfo, error)
	ChannelsList(context.Context, string, string, int) ([]teams.ChannelInfo, error)
}

type ProductionBackend struct {
	cfg *config.Config
}

func NewProductionBackend(cfg *config.Config) *ProductionBackend {
	return &ProductionBackend{cfg: cfg}
}

func (b *ProductionBackend) MailSearch(ctx context.Context, account, query string, limit int, topResults bool) ([]mail.SearchResultInfo, error) {
	return mail.SearchContext(ctx, b.cfg, account, query, limit, topResults)
}

func (b *ProductionBackend) MailGet(ctx context.Context, account, id string) (*mail.MessageInfo, error) {
	return mail.GetContext(ctx, b.cfg, account, id)
}

func (b *ProductionBackend) FilesSearch(ctx context.Context, account, query string, limit int) ([]storage.SearchResultInfo, error) {
	return storage.SearchContext(ctx, b.cfg, account, query, limit)
}

func (b *ProductionBackend) SharePointLibraries(ctx context.Context, account, teamID, siteID string, limit int) ([]storage.LibraryInfo, error) {
	if teamID != "" {
		return storage.ListTeamLibrariesContext(ctx, b.cfg, account, teamID, limit)
	}
	return storage.ListSiteLibrariesContext(ctx, b.cfg, account, siteID, limit)
}

func (b *ProductionBackend) DriveItemsList(ctx context.Context, account, driveID, itemID, path string, limit int) ([]storage.ItemInfo, error) {
	return storage.ListDriveContext(ctx, b.cfg, account, driveID, itemID, path, limit)
}

func (b *ProductionBackend) CalendarList(ctx context.Context, account string, from, to time.Time, search string) ([]cal.EventInfo, error) {
	return cal.ListContext(ctx, b.cfg, from, to, search, account, true)
}

func (b *ProductionBackend) TeamsList(ctx context.Context, account string, limit int) ([]teams.TeamInfo, error) {
	return teams.ListContext(ctx, b.cfg, account, limit)
}

func (b *ProductionBackend) ChannelsList(ctx context.Context, account, teamID string, limit int) ([]teams.ChannelInfo, error) {
	return teams.ListChannelsContext(ctx, b.cfg, account, teamID, limit)
}
