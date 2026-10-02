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
	Name       string `json:"name" untrusted:"onedrive_sharepoint,drive_item"`
	Type       string `json:"type"`
	MimeType   string `json:"mime_type,omitempty"`
	Size       int64  `json:"size,omitempty"`
	ChildCount int    `json:"child_count,omitempty"`
	Modified   string `json:"modified,omitempty"`
	WebURL     string `json:"web_url,omitempty"`
	ParentPath string `json:"parent_path,omitempty" untrusted:"onedrive_sharepoint,drive_item"`
	SiteID     string `json:"site_id,omitempty"`
}

type LibraryInfo struct {
	ID          string `json:"id"`
	Account     string `json:"account"`
	Name        string `json:"name" untrusted:"sharepoint,document_library"`
	Description string `json:"description,omitempty" untrusted:"sharepoint,document_library"`
	DriveType   string `json:"drive_type,omitempty"`
	WebURL      string `json:"web_url,omitempty"`
}

type SearchResultInfo struct {
	ItemInfo
	Rank    int    `json:"rank"`
	Summary string `json:"match_summary,omitempty" untrusted:"microsoft_search,drive_item_hit"`
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

func ListDrive(cfg *config.Config, account, driveID, itemID, path string, limit int) ([]ItemInfo, error) {
	if driveID == "" {
		return nil, fmt.Errorf("--drive-id is required")
	}
	client, err := clientFor(cfg, account)
	if err != nil {
		return nil, err
	}
	items, err := client.ListDriveChildren(driveID, itemID, path, limit)
	return convert(account, items), err
}

func ListTeamLibraries(cfg *config.Config, account, teamID string, limit int) ([]LibraryInfo, error) {
	if teamID == "" {
		return nil, fmt.Errorf("--team-id is required")
	}
	client, err := clientFor(cfg, account)
	if err != nil {
		return nil, err
	}
	drives, err := client.ListGroupDrives(teamID, limit)
	return convertLibraries(account, drives), err
}

func ListSiteLibraries(cfg *config.Config, account, siteID string, limit int) ([]LibraryInfo, error) {
	if siteID == "" {
		return nil, fmt.Errorf("--site-id is required")
	}
	client, err := clientFor(cfg, account)
	if err != nil {
		return nil, err
	}
	drives, err := client.ListSiteDrives(siteID, limit)
	return convertLibraries(account, drives), err
}

func Search(cfg *config.Config, account, query string, limit int) ([]SearchResultInfo, error) {
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	client, err := clientFor(cfg, account)
	if err != nil {
		return nil, err
	}
	hits, err := client.SearchDriveItems(query, limit)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResultInfo, 0, len(hits))
	for _, hit := range hits {
		results = append(results, SearchResultInfo{
			ItemInfo: convertItem(account, hit.Resource),
			Rank:     hit.Rank,
			Summary:  hit.Summary,
		})
	}
	return results, nil
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
		result = append(result, convertItem(account, item))
	}
	return result
}

func convertItem(account string, item graph.DriveItem) ItemInfo {
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
	return info
}

func convertLibraries(account string, drives []graph.Drive) []LibraryInfo {
	result := make([]LibraryInfo, 0, len(drives))
	for _, drive := range drives {
		result = append(result, LibraryInfo{
			ID: drive.ID, Account: account, Name: drive.Name,
			Description: drive.Description, DriveType: drive.DriveType, WebURL: drive.WebURL,
		})
	}
	return result
}
