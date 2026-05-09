package wal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andygonzalezhp/fastkv/internal/store"
)

type WAL struct {
	mu   sync.Mutex
	file *os.File
	path string
}

func Open(path string) (*WAL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &WAL{
		file: file,
		path: path,
	}, nil
}

func (w *WAL) Append(line string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := fmt.Fprintln(w.file, line); err != nil {
		return err
	}

	return w.file.Sync()
}

func (w *WAL) Close() error {
	return w.file.Close()
}

func Replay(path string, s *store.Store) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if err := applyLine(line, s); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	s.DeleteExpired()
	return nil
}

func applyLine(line string, s *store.Store) error {
	parts := strings.SplitN(line, " ", 3)
	command := strings.ToUpper(parts[0])

	switch command {
	case "SET":
		if len(parts) < 3 {
			return fmt.Errorf("invalid WAL SET command: %q", line)
		}

		key := parts[1]
		value := parts[2]
		s.Set(key, value)

	case "DEL":
		if len(parts) < 2 {
			return fmt.Errorf("invalid WAL DEL command: %q", line)
		}

		key := parts[1]
		s.Delete(key)

	case "EXPIREAT":
		if len(parts) < 3 {
			return fmt.Errorf("invalid WAL EXPIREAT command: %q", line)
		}

		key := parts[1]

		nanoTimestamp, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid EXPIREAT timestamp: %q", parts[2])
		}

		expiresAt := time.Unix(0, nanoTimestamp)
		s.ExpireAt(key, expiresAt)

	default:
		return fmt.Errorf("unknown WAL command: %q", command)
	}

	return nil
}

func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Truncate(0); err != nil {
		return err
	}

	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}

	return w.file.Sync()
}
