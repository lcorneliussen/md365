package teams

import (
	"fmt"

	"github.com/lcorneliussen/md365/internal/auth"
	"github.com/lcorneliussen/md365/internal/config"
	"github.com/lcorneliussen/md365/internal/graph"
)

type TeamInfo struct {
	ID          string `json:"id"`
	Account     string `json:"account"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
}

type ChannelInfo struct {
	ID              string `json:"id"`
	Account         string `json:"account"`
	TeamID          string `json:"team_id"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description,omitempty"`
	MembershipType  string `json:"membership_type,omitempty"`
	CreatedDateTime string `json:"created,omitempty"`
	WebURL          string `json:"web_url,omitempty"`
}

func List(cfg *config.Config, account string, limit int) ([]TeamInfo, error) {
	client, err := clientFor(cfg, account)
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
	if teamID == "" {
		return nil, fmt.Errorf("--team-id is required")
	}
	client, err := clientFor(cfg, account)
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
	if account == "" {
		return nil, fmt.Errorf("--account is required")
	}
	token, err := auth.GetAccessToken(cfg, account)
	if err != nil {
		return nil, err
	}
	return graph.NewClient(token), nil
}
