// Package store handles persistence of the command dictionary to disk as JSON.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Store represents the persisted name -> shell command dictionary.
type Store struct {
	path     string
	commands map[string]string
}

// configDirEnv, when set, overrides the resolved config directory.
// This is mainly useful for tests, which shouldn't touch the real
// user config directory.
const configDirEnv = "CL_CONFIG_DIR"

// ConfigDir returns the directory where cl stores its data files,
// creating it if it does not already exist.
func ConfigDir() (string, error) {
	dir := os.Getenv(configDirEnv)

	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config dir: %w", err)
		}
		dir = filepath.Join(base, "cl")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir %q: %w", dir, err)
	}

	return dir, nil
}

// commandsPath returns the full path to the commands JSON file.
func commandsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "commands.json"), nil
}

// Load reads the command dictionary from disk. If the file does not
// exist yet, it returns an empty, ready-to-use Store.
func Load() (*Store, error) {
	path, err := commandsPath()
	if err != nil {
		return nil, err
	}

	s := &Store{path: path, commands: map[string]string{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}

	if len(data) == 0 {
		return s, nil
	}

	if err := json.Unmarshal(data, &s.commands); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}

	return s, nil
}

// Save writes the current dictionary to disk atomically (write to a
// temp file, then rename) to avoid corrupting the file on crash.
func (s *Store) Save() error {
	data, err := json.MarshalIndent(s.commands, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal commands: %w", err)
	}

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "commands-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("rename temp file to %q: %w", s.path, err)
	}

	return nil
}

// Get returns the command for name and whether it exists.
func (s *Store) Get(name string) (string, bool) {
	cmd, ok := s.commands[name]
	return cmd, ok
}

// Set adds or overwrites the command for name.
func (s *Store) Set(name, command string) {
	s.commands[name] = command
}

// Remove deletes name from the dictionary. It returns false if name
// was not present.
func (s *Store) Remove(name string) bool {
	if _, ok := s.commands[name]; !ok {
		return false
	}
	delete(s.commands, name)
	return true
}

// Entry is a single name/command pair.
type Entry struct {
	Name    string
	Command string
}

// List returns all entries sorted alphabetically by name.
func (s *Store) List() []Entry {
	entries := make([]Entry, 0, len(s.commands))
	for name, cmd := range s.commands {
		entries = append(entries, Entry{Name: name, Command: cmd})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return entries
}
