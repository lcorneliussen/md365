package storage

import (
	"fmt"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/graph"
)

type ItemInfo struct {
	ID         string `json:"id"`
	DriveID    string `json:"drive_id,omitempty"`
	Account    string `json:"account"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	MimeType   string `json:"mime_type,omitempty"`
	Size       int64  `json:"size,omitempty"`
	ChildCount int    `json:"child_count,omitempty"`
	Modified   string `json:"modified,omitempty"`
	WebURL     string `json:"web_url,omitempty"`
	ParentPath string `json:"parent_path,omitempty"`
	SiteID     string `json:"site_id,omitempty"`
}

func ListOneDrive(cfg *config.Config, account, itemID, path string, limit int) ([]ItemInfo, error) {
	client, err := clientFor(cfg, account)
	if err != nil {
		return nil, err
	}
	items, err := client.ListMyDriveChildren(itemID, path, limit)
	return convert(account, items), err
}

func ListTeamDrive(cfg *config.Config, account, teamID, itemID, path string, limit int) ([]ItemInfo, error) {
	if teamID == "" {
		return nil, fmt.Errorf("--team-id is required")
	}
	client, err := clientFor(cfg, account)
	if err != nil {
		return nil, err
	}
	items, err := client.ListGroupDriveChildren(teamID, itemID, path, limit)
	return convert(account, items), err
}

func ListSiteDrive(cfg *config.Config, account, siteID, itemID, path string, limit int) ([]ItemInfo, error) {
	if siteID == "" {
		return nil, fmt.Errorf("--site-id is required")
	}
	client, err := clientFor(cfg, account)
	if err != nil {
		return nil, err
	}
	items, err := client.ListSiteDriveChildren(siteID, itemID, path, limit)
	return convert(account, items), err
}

func ListChannelFiles(cfg *config.Config, account, teamID, channelID, itemID string, limit int) ([]ItemInfo, error) {
	if teamID == "" || channelID == "" {
		return nil, fmt.Errorf("--team-id and --channel-id are required")
	}
	client, err := clientFor(cfg, account)
	if err != nil {
		return nil, err
	}
	folder, err := client.GetChannelFilesFolder(teamID, channelID)
	if err != nil {
		return nil, err
	}
	driveID := ""
	if folder.ParentReference != nil {
		driveID = folder.ParentReference.DriveID
	}
	if driveID == "" {
		return nil, fmt.Errorf("channel files folder did not include a drive ID")
	}
	if itemID == "" {
		itemID = folder.ID
	}
	items, err := client.ListDriveItemChildren(driveID, itemID, limit)
	return convert(account, items), err
}

func clientFor(cfg *config.Config, account string) (*graph.Client, error) {
	if account == "" {
		return nil, fmt.Errorf("--account is required")
	}
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return nil, err
	}
	return graph.NewClient(token), nil
}

func convert(account string, items []graph.DriveItem) []ItemInfo {
	result := make([]ItemInfo, 0, len(items))
	for _, item := range items {
		info := ItemInfo{
			ID: item.ID, Account: account, Name: item.Name, Size: item.Size,
			Modified: item.LastModifiedDateTime, WebURL: item.WebURL,
		}
		switch {
		case item.Folder != nil:
			info.Type = "folder"
			info.ChildCount = item.Folder.ChildCount
		case item.File != nil:
			info.Type = "file"
			info.MimeType = item.File.MimeType
		default:
			info.Type = "item"
		}
		if item.ParentReference != nil {
			info.DriveID = item.ParentReference.DriveID
			info.ParentPath = item.ParentReference.Path
			info.SiteID = item.ParentReference.SiteID
		}
		result = append(result, info)
	}
	return result
}
