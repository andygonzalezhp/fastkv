package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/andygonzalezhp/fastkv/internal/store"
)

type Server struct {
	addr  string
	store *store.Store
}

func NewServer(addr string, store *store.Store) *Server {
	return &Server{
		addr:  addr,
		store: store,
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

		if s.store.Delete(key) {
			return "1"
		}

		return "0"

	case "QUIT", "EXIT":
		return "BYE"

	default:
		return "ERR unknown command"

	case "EXPIRE":
		if len(parts) < 3 {
			return "ERR usage: EXPIRE <key> <seconds>"
		}

		key := parts[1]

		ttlSeconds, err := strconv.Atoi(parts[2])
		if err != nil || ttlSeconds <= 0 {
			return "ERR seconds must be a positive integer"
		}

		if s.store.Expire(key, ttlSeconds) {
			return "1"
		}

		return "0"

	case "TTL":
		if len(parts) < 2 {
			return "ERR usage: TTL <key>"
		}

		key := parts[1]

		ttl, _ := s.store.TTL(key)
		return strconv.Itoa(ttl)

	case "DBSIZE":
		return strconv.Itoa(s.store.Size())
	}
}
