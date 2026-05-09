package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/andygonzalezhp/fastkv/internal/store"
)

type Snapshot struct {
	CreatedAt time.Time              `json:"created_at"`
	Entries   map[string]store.Entry `json:"entries"`
}

func Save(path string, s *store.Store) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	snapshot := Snapshot{
		CreatedAt: time.Now(),
		Entries:   s.Snapshot(),
	}

	tmpPath := path + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(snapshot); err != nil {
		file.Close()
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

func Load(path string, s *store.Store) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer file.Close()

	var snapshot Snapshot

	if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
		return err
	}

	s.Load(snapshot.Entries)
	s.DeleteExpired()

	return nil
}
