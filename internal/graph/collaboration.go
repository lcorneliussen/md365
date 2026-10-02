package graph

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Team is a Microsoft Teams team the signed-in user has joined.
type Team struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	IsArchived  bool   `json:"isArchived,omitempty"`
	TenantID    string `json:"tenantId,omitempty"`
	WebURL      string `json:"webUrl,omitempty"`
}

// Channel is a channel within a Microsoft Teams team.
type Channel struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	Description     string `json:"description,omitempty"`
	MembershipType  string `json:"membershipType,omitempty"`
	CreatedDateTime string `json:"createdDateTime,omitempty"`
	WebURL          string `json:"webUrl,omitempty"`
}

// DriveItem is a file or folder in OneDrive or a SharePoint document library.
type DriveItem struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Size                 int64          `json:"size,omitempty"`
	WebURL               string         `json:"webUrl,omitempty"`
	CreatedDateTime      string         `json:"createdDateTime,omitempty"`
	LastModifiedDateTime string         `json:"lastModifiedDateTime,omitempty"`
	Folder               *FolderFacet   `json:"folder,omitempty"`
	File                 *FileFacet     `json:"file,omitempty"`
	ParentReference      *ItemReference `json:"parentReference,omitempty"`
}

type FolderFacet struct {
	ChildCount int `json:"childCount"`
}

type FileFacet struct {
	MimeType string `json:"mimeType,omitempty"`
}

type ItemReference struct {
	DriveID   string `json:"driveId,omitempty"`
	DriveType string `json:"driveType,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Path      string `json:"path,omitempty"`
	SiteID    string `json:"siteId,omitempty"`
}

const driveItemSelect = "id,name,size,webUrl,createdDateTime,lastModifiedDateTime,folder,file,parentReference"

// ListJoinedTeams returns teams the signed-in user is a direct member of.
func (c *Client) ListJoinedTeams(limit int) ([]Team, error) {
	reqURL := baseURL + "/me/joinedTeams"
	return listPaged[Team](c, reqURL, limit)
}

// ListChannels returns channels within a team.
func (c *Client) ListChannels(teamID string, limit int) ([]Channel, error) {
	reqURL := fmt.Sprintf("%s/teams/%s/channels", baseURL, url.PathEscape(teamID))
	return listPaged[Channel](c, reqURL, limit)
}

// GetChannelFilesFolder returns the SharePoint folder backing a Teams channel.
func (c *Client) GetChannelFilesFolder(teamID, channelID string) (*DriveItem, error) {
	reqURL := fmt.Sprintf("%s/teams/%s/channels/%s/filesFolder", baseURL, url.PathEscape(teamID), url.PathEscape(channelID))
	resp, err := c.doRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	var item DriveItem
	if err := json.Unmarshal(resp, &item); err != nil {
		return nil, fmt.Errorf("failed to parse channel files folder: %w", err)
	}
	return &item, nil
}

// ListMyDriveChildren lists a folder in the signed-in user's OneDrive.
func (c *Client) ListMyDriveChildren(itemID, path string, limit int) ([]DriveItem, error) {
	return c.listDriveChildren("me/drive", itemID, path, limit)
}

// ListGroupDriveChildren lists a folder in a Microsoft 365 group's default SharePoint library.
func (c *Client) ListGroupDriveChildren(groupID, itemID, path string, limit int) ([]DriveItem, error) {
	return c.listDriveChildren("groups/"+url.PathEscape(groupID)+"/drive", itemID, path, limit)
}

// ListSiteDriveChildren lists a folder in a SharePoint site's default document library.
func (c *Client) ListSiteDriveChildren(siteID, itemID, path string, limit int) ([]DriveItem, error) {
	return c.listDriveChildren("sites/"+url.PathEscape(siteID)+"/drive", itemID, path, limit)
}

// ListDriveItemChildren lists children for a known drive and item ID.
func (c *Client) ListDriveItemChildren(driveID, itemID string, limit int) ([]DriveItem, error) {
	reqURL := fmt.Sprintf("%s/drives/%s/items/%s/children", baseURL, url.PathEscape(driveID), url.PathEscape(itemID))
	return listDriveItems(c, reqURL, limit)
}

func (c *Client) listDriveChildren(prefix, itemID, path string, limit int) ([]DriveItem, error) {
	var reqURL string
	switch {
	case itemID != "":
		reqURL = fmt.Sprintf("%s/%s/items/%s/children", baseURL, prefix, url.PathEscape(itemID))
	case strings.Trim(path, "/") != "":
		reqURL = fmt.Sprintf("%s/%s/root:/%s:/children", baseURL, prefix, escapeDrivePath(path))
	default:
		reqURL = fmt.Sprintf("%s/%s/root/children", baseURL, prefix)
	}
	return listDriveItems(c, reqURL, limit)
}

func listDriveItems(c *Client, reqURL string, limit int) ([]DriveItem, error) {
	query := url.Values{}
	query.Set("$select", driveItemSelect)
	if limit > 0 {
		query.Set("$top", strconv.Itoa(min(limit, 999)))
	}
	reqURL += "?" + query.Encode()
	return listPaged[DriveItem](c, reqURL, limit)
}

func listPaged[T any](c *Client, reqURL string, limit int) ([]T, error) {
	if limit <= 0 {
		limit = 100
	}
	items := make([]T, 0, min(limit, 100))
	for reqURL != "" && len(items) < limit {
		resp, err := c.doRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		var page ODataResponse
		if err := json.Unmarshal(resp, &page); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		var values []T
		if err := json.Unmarshal(page.Value, &values); err != nil {
			return nil, fmt.Errorf("failed to parse collection: %w", err)
		}
		items = append(items, values...)
		reqURL = page.NextLink
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func escapeDrivePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
