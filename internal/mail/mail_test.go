package mail

import (
	"os"
	"strings"
	"testing"

	"github.com/lcorneliussen/md365/internal/config"
)

func TestPlanWriteNormalizesRecipientsAndEnforcesTenantWithoutGraph(t *testing.T) {
	cfg := &config.Config{Accounts: map[string]*config.Account{
		"talendos": {Domains: []string{"talendos.com"}},
		"oms":      {Domains: []string{"oms.example"}},
	}}
	recipients, err := PlanWrite(cfg, "talendos", " one@talendos.com;two@talendos.com, three@talendos.com ", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(recipients, ","); got != "one@talendos.com,two@talendos.com,three@talendos.com" {
		t.Fatalf("normalized recipients = %q", got)
	}
	if _, err := PlanWrite(cfg, "missing", "person@example.com", true); err == nil {
		t.Fatal("missing account accepted")
	}
	if _, err := PlanWrite(cfg, "talendos", "person@oms.example", false); err == nil {
		t.Fatal("cross-tenant recipient accepted")
	}
	if _, err := PlanWrite(cfg, "talendos", "person@oms.example", true); err != nil {
		t.Fatalf("explicit force rejected: %v", err)
	}
}

func TestValidateAccountRejectsRemovedAccountBeforeTokenLookup(t *testing.T) {
	cfg := &config.Config{Accounts: map[string]*config.Account{"talendos": {}}}
	if err := ValidateAccount(cfg, "talendos"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAccount(cfg, "removed"); err == nil {
		t.Fatal("removed account accepted")
	}
	if _, _, err := MarkRead(cfg, "removed", []string{"message-1"}); err == nil || !strings.Contains(err.Error(), "not found in config") {
		t.Fatalf("mark-read account validation = %v", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal filename", "document.pdf", "document.pdf"},
		{"with spaces", "my document.docx", "my document.docx"},
		{"path traversal slash", "../etc/passwd", "_etc_passwd"},
		{"path traversal backslash", "..\\windows\\system32", "_windows_system32"},
		{"null bytes", "file\x00name.txt", "filename.txt"},
		{"windows illegal chars", "file<>:|?*.txt", "file______.txt"},
		{"leading dot", ".hidden", "hidden"},
		{"trailing dot", "file.", "file"},
		{"leading space", " file.txt", "file.txt"},
		{"trailing space", "file.txt ", "file.txt"},
		{"all illegal", "<>:|?*", "______"},
		{"mixed unicode", "file名前.txt", "file名前.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNonConflictingPath(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		existing []string
		expected string
	}{
		{
			name:     "no conflict",
			path:     tmpDir + "/file.txt",
			existing: []string{},
			expected: tmpDir + "/file.txt",
		},
		{
			name:     "one conflict",
			path:     tmpDir + "/file.txt",
			existing: []string{tmpDir + "/file.txt"},
			expected: tmpDir + "/file (2).txt",
		},
		{
			name:     "multiple conflicts",
			path:     tmpDir + "/file.txt",
			existing: []string{tmpDir + "/file.txt", tmpDir + "/file (2).txt"},
			expected: tmpDir + "/file (3).txt",
		},
		{
			name:     "no extension",
			path:     tmpDir + "/file",
			existing: []string{tmpDir + "/file"},
			expected: tmpDir + "/file (2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create existing files
			for _, file := range tt.existing {
				if err := writeTestFile(file, "test"); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			}

			got := nonConflictingPath(tt.path)
			if got != tt.expected {
				t.Errorf("nonConflictingPath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func writeTestFile(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
