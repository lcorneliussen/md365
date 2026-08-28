package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lcorneliussen/md365/internal/output"
	"github.com/lcorneliussen/md365/skills"
	"github.com/spf13/cobra"
)

const (
	skillName       = "md365"
	skillFilename   = "SKILL.md"
	skillMarkerFile = ".md365-managed"
	skillMarkerText = "This skill is managed by md365. Manual edits may be overwritten by md365 skill install.\n"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print or install the embedded md365 agent skill",
	Long:  "Print or install the embedded SKILL.md that teaches coding agents how to use md365.",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := skills.FS.ReadFile("md365/SKILL.md")
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(data)
		return err
	},
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the md365 skill for coding agents",
	Long:  "Install the embedded md365 skill to ~/.agents/skills/md365 and detected Codex/Claude skill directories.",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := installSkill()
		if err != nil {
			return err
		}
		if writer.IsHuman() {
			for _, path := range paths {
				fmt.Fprintf(cmd.OutOrStdout(), "Installed md365 skill to %s\n", path)
			}
			return nil
		}
		return writeOK(paths,
			output.WithSummary("md365 skill installed"),
			output.WithBreadcrumbs(output.Breadcrumb{
				Action:      "show_skill",
				Command:     "md365 skill",
				Description: "Print the embedded skill",
			}),
		)
	},
}

func init() {
	skillCmd.AddCommand(skillInstallCmd)
}

func installSkill() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	data, err := skills.FS.ReadFile("md365/SKILL.md")
	if err != nil {
		return nil, err
	}

	targets := []string{
		filepath.Join(home, ".agents", "skills", skillName),
	}
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		targets = append(targets, filepath.Join(codexHome, "skills", skillName))
	} else if exists(filepath.Join(home, ".codex")) {
		targets = append(targets, filepath.Join(home, ".codex", "skills", skillName))
	}
	if exists(filepath.Join(home, ".claude")) {
		targets = append(targets, filepath.Join(home, ".claude", "skills", skillName))
	}

	installed := make([]string, 0, len(targets))
	for _, dir := range targets {
		path, err := writeManagedSkill(dir, data)
		if err != nil {
			return installed, err
		}
		installed = append(installed, path)
	}
	return installed, nil
}

func writeManagedSkill(dir string, data []byte) (string, error) {
	if err := claimManagedSkillDir(dir); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, skillMarkerFile), []byte(skillMarkerText), 0644); err != nil {
		return "", err
	}
	path := filepath.Join(dir, skillFilename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func claimManagedSkillDir(dir string) error {
	info, err := os.Lstat(dir)
	switch {
	case os.IsNotExist(err):
		return os.MkdirAll(dir, 0755)
	case err != nil:
		return err
	case info.Mode()&os.ModeSymlink != 0:
		return fmt.Errorf("%s is a symlink; refusing to overwrite unmanaged skill path", dir)
	case !info.IsDir():
		return fmt.Errorf("%s exists and is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if !exists(filepath.Join(dir, skillMarkerFile)) {
		return fmt.Errorf("%s exists but is not managed by md365; move it aside before installing", dir)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
