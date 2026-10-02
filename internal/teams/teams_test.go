package teams

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTeamInfoJSONDoesNotClaimUnsupportedJoinedTeamProperties(t *testing.T) {
	data, err := json.Marshal(TeamInfo{ID: "team-id", Account: "work", DisplayName: "Team"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	for _, field := range []string{"is_archived", "tenant_id", "web_url"} {
		if strings.Contains(string(data), field) {
			t.Errorf("TeamInfo JSON unexpectedly contains %q: %s", field, data)
		}
	}
}
