package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andygonzalezhp/fastkv/internal/snapshot"
	"github.com/andygonzalezhp/fastkv/internal/store"
	"github.com/andygonzalezhp/fastkv/internal/wal"
)

type Server struct {
	addr         string
	store        *store.Store
	wal          *wal.WAL
	snapshotPath string
	mu           sync.Mutex
}

func NewServer(addr string, store *store.Store, writeAheadLog *wal.WAL, snapshotPath string) *Server {
	return &Server{
		addr:         addr,
		store:        store,
		wal:          writeAheadLog,
		snapshotPath: snapshotPath,
	}
}

func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("FastKV server listening on %s\n", s.addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v\n", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		response := s.handleCommand(line)
		fmt.Fprintln(conn, response)

		if strings.EqualFold(line, "QUIT") || strings.EqualFold(line, "EXIT") {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("connection error: %v\n", err)
	}
}

func (s *Server) handleCommand(line string) string {
	parts := strings.SplitN(line, " ", 3)
	command := strings.ToUpper(parts[0])

	switch command {
	case "PING":
		return "PONG"

	case "SET":
		if len(parts) < 3 {
			return "ERR usage: SET <key> <value>"
		}

		key := parts[1]
		value := parts[2]

		s.mu.Lock()
		defer s.mu.Unlock()

		if err := s.wal.Append(fmt.Sprintf("SET %s %s", key, value)); err != nil {
			return "ERR failed to persist command"
		}

		s.store.Set(key, value)
		return "OK"

	case "GET":
		if len(parts) < 2 {
			return "ERR usage: GET <key>"
		}

		key := parts[1]

		value, ok := s.store.Get(key)
		if !ok {
			return "(nil)"
		}

		return value

	case "DEL":
		if len(parts) < 2 {
			return "ERR usage: DEL <key>"
		}

		key := parts[1]

		s.mu.Lock()
		defer s.mu.Unlock()

		if !s.store.Exists(key) {
			return "0"
		}

		if err := s.wal.Append(fmt.Sprintf("DEL %s", key)); err != nil {
			return "ERR failed to persist command"
		}

		s.store.Delete(key)
		return "1"

	case "EXPIRE":
		if len(parts) < 3 {
			return "ERR usage: EXPIRE <key> <seconds>"
		}

		key := parts[1]

		ttlSeconds, err := strconv.Atoi(parts[2])
		if err != nil || ttlSeconds <= 0 {
			return "ERR seconds must be a positive integer"
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		if !s.store.Exists(key) {
			return "0"
		}

		expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

		if err := s.wal.Append(fmt.Sprintf("EXPIREAT %s %d", key, expiresAt.UnixNano())); err != nil {
			return "ERR failed to persist command"
		}

		s.store.ExpireAt(key, expiresAt)
		return "1"

	case "TTL":
		if len(parts) < 2 {
			return "ERR usage: TTL <key>"
		}

		key := parts[1]

		ttl, _ := s.store.TTL(key)
		return strconv.Itoa(ttl)

	case "DBSIZE":
		return strconv.Itoa(s.store.Size())

	case "STATS":
		stats := s.store.Stats()

		maxKeys := strconv.Itoa(stats.MaxKeys)
		if stats.MaxKeys == 0 {
			maxKeys = "unlimited"
		}

		return fmt.Sprintf(
			"keys=%d max_keys=%s eviction_policy=%s evictions=%d",
			stats.Keys,
			maxKeys,
			stats.EvictionPolicy,
			stats.Evictions,
		)

	case "SAVE":
		s.mu.Lock()
		defer s.mu.Unlock()

		s.store.DeleteExpired()

		if err := snapshot.Save(s.snapshotPath, s.store); err != nil {
			return "ERR failed to save snapshot"
		}

		if err := s.wal.Truncate(); err != nil {
			return "ERR failed to truncate WAL"
		}

		return "OK"

	case "QUIT", "EXIT":
		return "BYE"

	default:
		return "ERR unknown command"
	}
}
