package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectSkillLinks makes a task-private CLI skills directory contain only
// symlinks to the already prepared run Skill view.
func ProjectSkillLinks(sourceDir, targetDir string) error {
	if sourceDir == "" {
		return nil
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("read run skills: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create task CLI skills directory: %w", err)
	}
	// The directory belongs to this task-private runtime config. Remove only
	// old symlinks so a reused task directory cannot expose a prior run's Skill.
	previous, err := os.ReadDir(targetDir)
	if err != nil {
		return fmt.Errorf("read task CLI skills directory: %w", err)
	}
	for _, entry := range previous {
		path := filepath.Join(targetDir, entry.Name())
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("refuse to replace non-symlink %s", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		source := filepath.Join(sourceDir, entry.Name())
		info, statErr := os.Stat(source)
		if statErr != nil || !info.IsDir() {
			continue
		}
		target := filepath.Join(targetDir, entry.Name())
		if err := os.Symlink(source, target); err != nil {
			return err
		}
	}
	return nil
}
