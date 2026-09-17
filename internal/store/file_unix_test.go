//go:build !windows

package store

import (
	"os"
	"testing"
)

// Windows has no permission bits: os.Stat reports every file as 0666 no matter
// what mode Save asked for, so there is nothing to assert there.
func TestSaveSetsPrivatePermissions(t *testing.T) {
	s := &Store{Path: t.TempDir() + "/nested/orgmaid.org"}
	s.Journal.EnsureDay("2026-08-01")

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(s.Path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("file mode = %v, want no group or other access", info.Mode())
	}
}
