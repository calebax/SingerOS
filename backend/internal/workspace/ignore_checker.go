package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ---- Ignore rules (extracted from scanner.go) ----

// IgnoreChecker applies gitignore rules and Leros built-in excludes.
type IgnoreChecker struct {
	repoDir  string
	gitDir   string
	lerosDir string
	cache    map[string]bool
	useGit   bool
}

// NewIgnoreChecker creates an IgnoreChecker for the given repo directory.
func NewIgnoreChecker(repoDir string) (*IgnoreChecker, error) {
	absRepo, err := filepath.Abs(repoDir)
	if err != nil {
		return nil, fmt.Errorf("resolve repo dir: %w", err)
	}
	gitDir := filepath.Join(absRepo, ".git")
	lerosDir := filepath.Join(absRepo, ".leros")
	_, gitErr := os.Stat(gitDir)
	useGit := gitErr == nil
	return &IgnoreChecker{
		repoDir:  absRepo,
		gitDir:   gitDir,
		lerosDir: lerosDir,
		cache:    make(map[string]bool),
		useGit:   useGit,
	}, nil
}

// ShouldSkipDir returns true when a directory should be excluded from scanning entirely.
func (c *IgnoreChecker) ShouldSkipDir(absPath string) bool {
	if absPath == c.gitDir || absPath == c.lerosDir {
		return true
	}
	if c.isBuiltinExcludedDir(absPath) {
		return true
	}
	return false
}

func (c *IgnoreChecker) isBuiltinExcludedDir(absPath string) bool {
	name := filepath.Base(absPath)
	switch strings.ToLower(name) {
	case "tmp", "temp", "logs", "log", ".cache", "node_modules", "vendor", "dist", "build", "target":
		return true
	}
	return false
}

// IsIgnored returns true when a file should be excluded from scanning.
func (c *IgnoreChecker) IsIgnored(absPath string) (bool, error) {
	if cached, ok := c.cache[absPath]; ok {
		return cached, nil
	}
	// Built-in filename exclusions
	if c.isBuiltinExcludedFile(absPath) {
		c.cache[absPath] = true
		return true, nil
	}
	// Git check-ignore via stdin
	if c.useGit {
		ignored, err := c.gitCheckIgnore(absPath)
		if err == nil {
			c.cache[absPath] = ignored
			return ignored, nil
		}
	}
	c.cache[absPath] = false
	return false, nil
}

func (c *IgnoreChecker) isBuiltinExcludedFile(absPath string) bool {
	base := filepath.Base(absPath)
	if base == ".DS_Store" || base == "Thumbs.db" || base == ".gitignore" {
		return true
	}
	ext := filepath.Ext(base)
	if ext == ".swp" || ext == ".swo" || ext == ".log" {
		return true
	}
	return false
}

func (c *IgnoreChecker) gitCheckIgnore(absPath string) (bool, error) {
	rel, err := filepath.Rel(c.repoDir, absPath)
	if err != nil {
		return false, err
	}
	rel = filepath.ToSlash(rel)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "check-ignore", "--stdin", "--no-index")
	cmd.Dir = c.repoDir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false, err
	}
	go func() {
		defer stdin.Close()
		fmt.Fprintln(stdin, rel)
	}()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 1: path is NOT ignored
			// Exit code 128: git error
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, err
	}
	return len(bytes.TrimSpace(output)) > 0, nil
}

// ---- Manifest helpers (extracted from scanner.go) ----

// manifestHasFinalEntries checks whether the manifest already contains at least one final entry.
func manifestHasFinalEntries(manifestPath string) (bool, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("open manifest: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry ManifestArtifact
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.IsFinal {
			return true, nil
		}
	}
	return false, scanner.Err()
}
