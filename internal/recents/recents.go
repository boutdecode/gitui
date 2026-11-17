// Package recents remembers the repositories the user opened last.
package recents

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/kevin/gitui/internal/domain"
)

// Max is the number of repositories kept.
const Max = 20

// Store is a small JSON file of recently opened repositories, most recent
// first. A Store with an empty Path keeps everything in memory only. It is
// safe for concurrent use.
type Store struct {
	Path string

	mu    sync.Mutex
	repos []domain.Repo
	read  bool
}

func New(path string) *Store { return &Store{Path: path} }

// List returns the remembered repositories, most recent first. An unreadable
// or corrupt file counts as empty: recents are a convenience, never an error.
func (s *Store) List() []domain.Repo {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return append([]domain.Repo(nil), s.repos...)
}

// Add moves repo to the front and persists the list.
func (s *Store) Add(repo domain.Repo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	out := []domain.Repo{repo}
	for _, r := range s.repos {
		if r.FullName() != repo.FullName() && len(out) < Max {
			out = append(out, r)
		}
	}
	s.repos = out
	return s.save()
}

func (s *Store) load() {
	if s.read {
		return
	}
	s.read = true
	if s.Path == "" {
		return
	}
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return
	}
	var rs []domain.Repo
	if json.Unmarshal(b, &rs) == nil {
		s.repos = rs
	}
}

func (s *Store) save() error {
	if s.Path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(s.repos)
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return os.Rename(tmp, s.Path)
}
