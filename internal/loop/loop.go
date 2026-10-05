package loop

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/graph"
)

// ComponentInfo is a Microsoft Loop component returned by Microsoft Search.
type ComponentInfo struct {
	ID         string `json:"id"`
	DriveID    string `json:"drive_id,omitempty"`
	SiteID     string `json:"site_id,omitempty"`
	Account    string `json:"account"`
	Name       string `json:"name" untrusted:"microsoft_loop,loop_component"`
	Format     string `json:"format"`
	MimeType   string `json:"mime_type,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Modified   string `json:"modified,omitempty"`
	WebURL     string `json:"web_url,omitempty"`
	ParentPath string `json:"parent_path,omitempty" untrusted:"microsoft_loop,loop_component"`
	Rank       int    `json:"rank"`
	Summary    string `json:"match_summary,omitempty" untrusted:"microsoft_search,loop_component_hit"`
}

func Search(cfg *config.Config, account, query string, limit int) ([]ComponentInfo, error) {
	return SearchContext(context.Background(), cfg, account, query, limit)
}

func SearchContext(ctx context.Context, cfg *config.Config, account, query string, limit int) ([]ComponentInfo, error) {
	if strings.TrimSpace(account) == "" {
		return nil, fmt.Errorf("--account is required")
	}
	token, err := auth.GetAccessTokenContext(ctx, cfg, account)
	if err != nil {
		return nil, err
	}
	hits, err := graph.NewClientWithContext(ctx, token).SearchLoopComponents(query, limit)
	if err != nil {
		return nil, err
	}
	values := make([]ComponentInfo, 0, len(hits))
	for _, hit := range hits {
		item := hit.Resource
		value := ComponentInfo{
			ID: item.ID, Account: account, Name: item.Name, Format: componentFormat(item.Name),
			Size: item.Size, Modified: item.LastModifiedDateTime, WebURL: item.WebURL,
			Rank: hit.Rank, Summary: hit.Summary,
		}
		if item.File != nil {
			value.MimeType = item.File.MimeType
		}
		if item.ParentReference != nil {
			value.DriveID = item.ParentReference.DriveID
			value.SiteID = item.ParentReference.SiteID
			value.ParentPath = item.ParentReference.Path
		}
		values = append(values, value)
	}
	return values, nil
}

func componentFormat(name string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "fluid" {
		return "fluid"
	}
	return "loop"
}
