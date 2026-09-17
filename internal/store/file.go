package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Store is a journal bound to the file it lives in.
type Store struct {
	Path    string
	Journal Journal
}

// DefaultPath is ~/.orgmaid/orgmaid.org, resolved through the platform home directory.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".orgmaid", "orgmaid.org"), nil
}

// Open loads path. A file that does not exist yet yields an empty journal
// rather than an error, so first run needs no bootstrap step.
func Open(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Store{Path: path}, nil
	}
	if err != nil {
		return nil, err
	}
	return &Store{Path: path, Journal: Parse(data)}, nil
}

// Save writes the journal out wholesale, replacing the file atomically so a
// crash mid-write cannot truncate the log.
func (s *Store) Save() error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".orgmaid-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(s.Journal.Serialize()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, s.Path)
}

// Day returns the day for date, creating an empty one when the journal has none.
func (s *Store) Day(date string) *Day {
	return &s.Journal.Days[s.Journal.EnsureDay(date)]
}
