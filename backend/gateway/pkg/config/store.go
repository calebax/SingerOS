package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvCredentialStore implements onboard.CredentialStore backed by a .env file.
//
// This mirrors Hermes' pattern of storing platform credentials in an env file
// separate from the main config.yaml. Sensitive values (app_id, client_secret,
// bot_token) live here; non-sensitive policy config (group_policy, allowed_users)
// lives in config.yaml.
//
// The env file uses standard KEY=VALUE format, one per line. Platform-specific
// keys are prefixed with the uppercase platform code (e.g., QQBOT_APP_ID).
type EnvCredentialStore struct {
	path string
}

// NewEnvCredentialStore creates a credential store backed by the given file.
//
// The directory will be created if it does not exist. The default path is
// $HOME/.singeros/credentials.env.
func NewEnvCredentialStore(path string) (*EnvCredentialStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create credential dir %s: %w", dir, err)
	}
	return &EnvCredentialStore{path: path}, nil
}

// Save persists platform credentials to the env file.
//
// Keys are written as {PLATFORM}_{KEY}=value. For example:
//
//	QQBOT_APP_ID=12345
//	QQBOT_CLIENT_SECRET=abcdef
//
// The file is locked for concurrent safety via atomic write (write to temp,
// then rename).
func (s *EnvCredentialStore) Save(platform string, credentials map[string]string) error {
	// Read existing entries
	existing, err := s.readAll()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read existing credentials: %w", err)
	}
	if existing == nil {
		existing = make(map[string]string)
	}

	// Merge new credentials
	prefix := strings.ToUpper(platform) + "_"
	for k, v := range credentials {
		key := prefix + strings.ToUpper(k)
		existing[key] = v
	}

	// Also inject environment variables
	for k, v := range credentials {
		key := prefix + strings.ToUpper(k)
		os.Setenv(key, v)
	}

	// Write atomically
	return s.writeAll(existing)
}

// Load retrieves platform credentials from the env file.
func (s *EnvCredentialStore) Load(platform string) (map[string]string, error) {
	all, err := s.readAll()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	prefix := strings.ToUpper(platform) + "_"
	result := make(map[string]string)
	for k, v := range all {
		if strings.HasPrefix(k, prefix) {
			key := strings.ToLower(strings.TrimPrefix(k, prefix))
			result[key] = v
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// Remove deletes all credentials for a platform.
func (s *EnvCredentialStore) Remove(platform string) error {
	all, err := s.readAll()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	prefix := strings.ToUpper(platform) + "_"
	for k := range all {
		if strings.HasPrefix(k, prefix) {
			delete(all, k)
			os.Unsetenv(k)
		}
	}

	return s.writeAll(all)
}

// Exists returns true if credentials exist for the platform.
func (s *EnvCredentialStore) Exists(platform string) bool {
	creds, _ := s.Load(platform)
	return len(creds) > 0
}

// readAll reads all key-value pairs from the env file.
func (s *EnvCredentialStore) readAll() (map[string]string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result, nil
}

// writeAll writes all key-value pairs atomically to the env file.
func (s *EnvCredentialStore) writeAll(entries map[string]string) error {
	var lines []string
	for k, v := range entries {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}

	content := "# SingerOS Gateway Credentials\n# Auto-generated — do not edit manually\n\n" +
		strings.Join(lines, "\n") + "\n"

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	return os.Rename(tmpPath, s.path)
}

// DefaultCredentialPath returns the default credential file path.
func DefaultCredentialPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".singeros-credentials.env"
	}
	return filepath.Join(home, ".singeros", "credentials.env")
}
