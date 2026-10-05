package graph

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Team is a Microsoft Teams team the signed-in user has joined.
type Team struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
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

// Drive is a OneDrive or SharePoint document library.
type Drive struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DriveType   string `json:"driveType,omitempty"`
	WebURL      string `json:"webUrl,omitempty"`
}

// DriveItemSearchHit is a ranked Microsoft Search result.
type DriveItemSearchHit struct {
	HitID    string    `json:"hitId"`
	Rank     int       `json:"rank"`
	Summary  string    `json:"summary,omitempty"`
	Resource DriveItem `json:"resource"`
}

// MessageSearchHit is a Microsoft Search result from the signed-in user's
// Exchange Online mailbox.
type MessageSearchHit struct {
	HitID    string  `json:"hitId"`
	Rank     int     `json:"rank"`
	Summary  string  `json:"summary,omitempty"`
	Resource Message `json:"resource"`
}

type searchRequest struct {
	Requests []searchQuery `json:"requests"`
}

type searchQuery struct {
	EntityTypes      []string        `json:"entityTypes"`
	Query            searchQueryText `json:"query"`
	From             int             `json:"from"`
	Size             int             `json:"size"`
	Fields           []string        `json:"fields"`
	EnableTopResults bool            `json:"enableTopResults,omitempty"`
}

type messageSearchResponse struct {
	Value []struct {
		HitsContainers []struct {
			Hits                 []MessageSearchHit `json:"hits"`
			MoreResultsAvailable bool               `json:"moreResultsAvailable"`
		} `json:"hitsContainers"`
	} `json:"value"`
}

type searchQueryText struct {
	QueryString string `json:"queryString"`
}

type searchResponse struct {
	Value []struct {
		HitsContainers []struct {
			Hits                 []DriveItemSearchHit `json:"hits"`
			MoreResultsAvailable bool                 `json:"moreResultsAvailable"`
		} `json:"hitsContainers"`
	} `json:"value"`
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

// ListDriveChildren lists a folder in a known OneDrive or SharePoint document library.
func (c *Client) ListDriveChildren(driveID, itemID, path string, limit int) ([]DriveItem, error) {
	return c.listDriveChildren("drives/"+url.PathEscape(driveID), itemID, path, limit)
}

// ListGroupDrives lists every document library associated with a Microsoft 365 group.
func (c *Client) ListGroupDrives(groupID string, limit int) ([]Drive, error) {
	return c.listDrives("groups/"+url.PathEscape(groupID)+"/drives", limit)
}

// ListSiteDrives lists every document library associated with a SharePoint site.
func (c *Client) ListSiteDrives(siteID string, limit int) ([]Drive, error) {
	return c.listDrives("sites/"+url.PathEscape(siteID)+"/drives", limit)
}

// ListDriveItemChildren lists children for a known drive and item ID.
func (c *Client) ListDriveItemChildren(driveID, itemID string, limit int) ([]DriveItem, error) {
	return c.ListDriveChildren(driveID, itemID, "", limit)
}

// GetDriveItem returns a drive item with reliable file/folder facets.
func (c *Client) GetDriveItem(driveID, itemID string) (*DriveItem, error) {
	query := url.Values{}
	query.Set("$select", driveItemSelect)
	reqURL := fmt.Sprintf("%s/drives/%s/items/%s?%s", baseURL, url.PathEscape(driveID), url.PathEscape(itemID), query.Encode())
	resp, err := c.doRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	var item DriveItem
	if err := json.Unmarshal(resp, &item); err != nil {
		return nil, fmt.Errorf("failed to parse drive item: %w", err)
	}
	return &item, nil
}

// SearchDriveItems searches all OneDrive and SharePoint content visible to the signed-in user.
func (c *Client) SearchDriveItems(query string, limit int) ([]DriveItemSearchHit, error) {
	return c.searchDriveItems(baseURL+"/search/query", query, limit, nil)
}

// SearchLoopComponents searches Microsoft Loop components that are visible to
// Microsoft Search. Loop components are current .loop files or legacy .fluid
// files stored in OneDrive, SharePoint, or indexed SharePoint Embedded storage.
func (c *Client) SearchLoopComponents(query string, limit int) ([]DriveItemSearchHit, error) {
	return c.searchDriveItems(baseURL+"/search/query", loopSearchQuery(query), limit, func(hit DriveItemSearchHit) bool {
		return isLoopComponentName(hit.Resource.Name)
	})
}

func (c *Client) searchDriveItems(endpoint, query string, limit int, accept func(DriveItemSearchHit) bool) ([]DriveItemSearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	if limit <= 0 {
		limit = 25
	}

	results := make([]DriveItemSearchHit, 0, min(limit, 100))
	for from, scanned := 0, 0; len(results) < limit; {
		size := min(limit-len(results), 100)
		body, err := json.Marshal(newDriveItemSearchRequest(query, from, size))
		if err != nil {
			return nil, fmt.Errorf("failed to encode search request: %w", err)
		}
		resp, err := c.doRequest("POST", endpoint, body)
		if err != nil {
			return nil, err
		}
		hits, more, err := parseDriveItemSearchResponse(resp)
		if err != nil {
			return nil, err
		}
		scanned += len(hits)
		for _, hit := range hits {
			if accept == nil || accept(hit) {
				results = append(results, hit)
				if len(results) == limit {
					break
				}
			}
		}
		if !more || len(hits) == 0 {
			break
		}
		if scanned >= 10000 {
			return nil, fmt.Errorf("Microsoft Search scanned 10000 drive items without completing the requested result set; narrow the query")
		}
		from += size
	}
	if len(results) > limit {
		results = results[:limit]
	}
	c.hydrateAmbiguousSearchHits(results)
	return results, nil
}

func isLoopComponentName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(name, ".loop") || strings.HasSuffix(name, ".fluid")
}

func loopSearchQuery(query string) string {
	const types = "(filetype:loop OR filetype:fluid)"
	query = strings.TrimSpace(query)
	if query == "" {
		return types
	}
	return "(" + query + ") AND " + types
}

// SearchMessages searches the signed-in user's Exchange Online mailbox. By
// default Microsoft Search returns newest-first results; topResults promotes
// the most relevant matches at the start of the result set.
func (c *Client) SearchMessages(query string, limit int, topResults bool) ([]MessageSearchHit, error) {
	return c.searchMessages(baseURL+"/search/query", query, limit, topResults)
}

func (c *Client) searchMessages(endpoint, query string, limit int, topResults bool) ([]MessageSearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	if limit <= 0 {
		limit = 25
	}

	results := make([]MessageSearchHit, 0, min(limit, 25))
	for from := 0; len(results) < limit; {
		size := min(limit-len(results), 25)
		body, err := json.Marshal(newMessageSearchRequest(query, from, size, topResults))
		if err != nil {
			return nil, fmt.Errorf("failed to encode message search request: %w", err)
		}
		resp, err := c.doRequest("POST", endpoint, body)
		if err != nil {
			return nil, err
		}
		hits, more, err := parseMessageSearchResponse(resp)
		if err != nil {
			return nil, err
		}
		results = append(results, hits...)
		if !more || len(hits) == 0 {
			break
		}
		from += size
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
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

func (c *Client) listDrives(prefix string, limit int) ([]Drive, error) {
	query := url.Values{}
	query.Set("$select", "id,name,description,driveType,webUrl")
	if limit > 0 {
		query.Set("$top", strconv.Itoa(min(limit, 999)))
	}
	return listPaged[Drive](c, baseURL+"/"+prefix+"?"+query.Encode(), limit)
}

func newDriveItemSearchRequest(query string, from, size int) searchRequest {
	return searchRequest{Requests: []searchQuery{{
		EntityTypes: []string{"driveItem"},
		Query:       searchQueryText{QueryString: query},
		From:        from,
		Size:        size,
		Fields: []string{
			"id", "name", "size", "webUrl", "createdDateTime", "lastModifiedDateTime",
			"folder", "file", "parentReference",
		},
	}}}
}

func newMessageSearchRequest(query string, from, size int, topResults bool) searchRequest {
	return searchRequest{Requests: []searchQuery{{
		EntityTypes:      []string{"message"},
		Query:            searchQueryText{QueryString: query},
		From:             from,
		Size:             size,
		EnableTopResults: topResults,
		Fields: []string{
			"id", "subject", "from", "toRecipients", "ccRecipients",
			"receivedDateTime", "sentDateTime", "isRead", "hasAttachments",
			"bodyPreview", "conversationId", "webLink",
		},
	}}}
}

func parseDriveItemSearchResponse(data []byte) ([]DriveItemSearchHit, bool, error) {
	var response searchResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, false, fmt.Errorf("failed to parse search response: %w", err)
	}
	var hits []DriveItemSearchHit
	more := false
	for _, value := range response.Value {
		for _, container := range value.HitsContainers {
			hits = append(hits, container.Hits...)
			more = more || container.MoreResultsAvailable
		}
	}
	return hits, more, nil
}

func parseMessageSearchResponse(data []byte) ([]MessageSearchHit, bool, error) {
	var response messageSearchResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, false, fmt.Errorf("failed to parse message search response: %w", err)
	}
	var hits []MessageSearchHit
	more := false
	for _, value := range response.Value {
		for _, container := range value.HitsContainers {
			hits = append(hits, container.Hits...)
			more = more || container.MoreResultsAvailable
		}
	}
	return hits, more, nil
}

func (c *Client) hydrateAmbiguousSearchHits(hits []DriveItemSearchHit) {
	var wait sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i := range hits {
		item := hits[i].Resource
		if !ambiguousSearchItem(item) || item.ParentReference == nil || item.ParentReference.DriveID == "" {
			continue
		}
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			item, err := c.GetDriveItem(hits[index].Resource.ParentReference.DriveID, hits[index].Resource.ID)
			if err == nil {
				hits[index].Resource = *item
				return
			}
			// Microsoft Search represents some folders as zero-byte octet-stream
			// files. If hydration fails, avoid publishing a known-wrong type.
			hits[index].Resource.File = nil
		}(i)
	}
	wait.Wait()
}

func ambiguousSearchItem(item DriveItem) bool {
	return item.Folder == nil && item.File != nil && item.Size == 0 && item.File.MimeType == "application/octet-stream"
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
