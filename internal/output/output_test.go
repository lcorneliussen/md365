package output

import (
	"testing"

	"github.com/lcorneliussen/md365/internal/apierr"
)

func TestPolicyDeniedExitCode(t *testing.T) {
	if got := ExitCodeFor(apierr.Policy("blocked")); got != 8 {
		t.Fatalf("policy denied exit code = %d, want 8", got)
	}
}
