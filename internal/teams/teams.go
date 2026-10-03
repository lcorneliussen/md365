package teams

import (
	"context"
	"fmt"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/graph"
)

type TeamInfo struct {
	ID          string `json:"id"`
	Account     string `json:"account"`
	DisplayName string `json:"display_name" untrusted:"microsoft_teams,team"`
	Description string `json:"description,omitempty" untrusted:"microsoft_teams,team"`
}

type ChannelInfo struct {
	ID              string `json:"id"`
	Account         string `json:"account"`
	TeamID          string `json:"team_id"`
	DisplayName     string `json:"display_name" untrusted:"microsoft_teams,channel"`
	Description     string `json:"description,omitempty" untrusted:"microsoft_teams,channel"`
	MembershipType  string `json:"membership_type,omitempty"`
	CreatedDateTime string `json:"created,omitempty"`
	WebURL          string `json:"web_url,omitempty"`
}

func List(cfg *config.Config, account string, limit int) ([]TeamInfo, error) {
	return ListContext(context.Background(), cfg, account, limit)
}

func ListContext(ctx context.Context, cfg *config.Config, account string, limit int) ([]TeamInfo, error) {
	client, err := clientForContext(ctx, cfg, account)
	if err != nil {
		return nil, err
	}
	values, err := client.ListJoinedTeams(limit)
	if err != nil {
		return nil, err
	}
	result := make([]TeamInfo, 0, len(values))
	for _, value := range values {
		result = append(result, TeamInfo{
			ID: value.ID, Account: account, DisplayName: value.DisplayName,
			Description: value.Description,
		})
	}
	return result, nil
}

func ListChannels(cfg *config.Config, account, teamID string, limit int) ([]ChannelInfo, error) {
	return ListChannelsContext(context.Background(), cfg, account, teamID, limit)
}

func ListChannelsContext(ctx context.Context, cfg *config.Config, account, teamID string, limit int) ([]ChannelInfo, error) {
	if teamID == "" {
		return nil, fmt.Errorf("--team-id is required")
	}
	client, err := clientForContext(ctx, cfg, account)
	if err != nil {
		return nil, err
	}
	values, err := client.ListChannels(teamID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]ChannelInfo, 0, len(values))
	for _, value := range values {
		result = append(result, ChannelInfo{
			ID: value.ID, Account: account, TeamID: teamID, DisplayName: value.DisplayName,
			Description: value.Description, MembershipType: value.MembershipType,
			CreatedDateTime: value.CreatedDateTime, WebURL: value.WebURL,
		})
	}
	return result, nil
}

func clientFor(cfg *config.Config, account string) (*graph.Client, error) {
	return clientForContext(context.Background(), cfg, account)
}

func clientForContext(ctx context.Context, cfg *config.Config, account string) (*graph.Client, error) {
	if account == "" {
		return nil, fmt.Errorf("--account is required")
	}
	token, err := auth.GetAccessTokenContext(ctx, cfg, account)
	if err != nil {
		return nil, err
	}
	return graph.NewClientWithContext(ctx, token), nil
}
