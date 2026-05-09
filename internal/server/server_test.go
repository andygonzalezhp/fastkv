package server

import (
	"bufio"
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andygonzalezhp/fastkv/internal/snapshot"
	"github.com/andygonzalezhp/fastkv/internal/store"
	"github.com/andygonzalezhp/fastkv/internal/wal"
)

func TestTCPBasicCommands(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, 0, filepath.Join(dir, "fastkv.wal"), filepath.Join(dir, "fastkv.snapshot"))
	defer cleanup()

	responses := runCommands(t, addr,
		"PING",
		"SET name andy",
		"GET name",
		"DEL name",
		"GET name",
		"QUIT",
	)

	expected := []string{
		"PONG",
		"OK",
		"andy",
		"1",
		"(nil)",
		"BYE",
	}

	assertResponses(t, responses, expected)
}

func TestTCPTTLExpiration(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, 0, filepath.Join(dir, "fastkv.wal"), filepath.Join(dir, "fastkv.snapshot"))
	defer cleanup()

	responses := runCommands(t, addr,
		"SET temp hello",
		"EXPIRE temp 1",
		"GET temp",
	)

	expected := []string{
		"OK",
		"1",
		"hello",
	}

	assertResponses(t, responses, expected)

	time.Sleep(1200 * time.Millisecond)

	responses = runCommands(t, addr,
		"GET temp",
		"TTL temp",
	)

	expected = []string{
		"(nil)",
		"-2",
	}

	assertResponses(t, responses, expected)
}

func TestTCPLRUEviction(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, 2, filepath.Join(dir, "fastkv.wal"), filepath.Join(dir, "fastkv.snapshot"))
	defer cleanup()

	responses := runCommands(t, addr,
		"SET a 1",
		"SET b 2",
		"GET a",
		"SET c 3",
		"GET b",
		"GET a",
		"GET c",
		"STATS",
	)

	expected := []string{
		"OK",
		"OK",
		"1",
		"OK",
		"(nil)",
		"1",
		"3",
		"keys=2 max_keys=2 eviction_policy=lru evictions=1",
	}

	assertResponses(t, responses, expected)
}

func TestTCPWALRecovery(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "fastkv.wal")
	snapshotPath := filepath.Join(dir, "fastkv.snapshot")

	addr, cleanup := startTestServer(t, 0, walPath, snapshotPath)

	responses := runCommands(t, addr,
		"SET name andy",
		"SET language go",
		"QUIT",
	)

	expected := []string{
		"OK",
		"OK",
		"BYE",
	}

	assertResponses(t, responses, expected)
	cleanup()

	addr, cleanup = startTestServer(t, 0, walPath, snapshotPath)
	defer cleanup()

	responses = runCommands(t, addr,
		"GET name",
		"GET language",
		"DBSIZE",
	)

	expected = []string{
		"andy",
		"go",
		"2",
	}

	assertResponses(t, responses, expected)
}

func TestTCPSnapshotRecovery(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "fastkv.wal")
	snapshotPath := filepath.Join(dir, "fastkv.snapshot")

	addr, cleanup := startTestServer(t, 0, walPath, snapshotPath)

	responses := runCommands(t, addr,
		"SET name andy",
		"SET language go",
		"SAVE",
		"QUIT",
	)

	expected := []string{
		"OK",
		"OK",
		"OK",
		"BYE",
	}

	assertResponses(t, responses, expected)
	cleanup()

	addr, cleanup = startTestServer(t, 0, walPath, snapshotPath)
	defer cleanup()

	responses = runCommands(t, addr,
		"GET name",
		"GET language",
		"DBSIZE",
	)

	expected = []string{
		"andy",
		"go",
		"2",
	}

	assertResponses(t, responses, expected)
}

func startTestServer(t *testing.T, maxKeys int, walPath string, snapshotPath string) (string, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	kvStore := store.NewStoreWithMaxKeys(maxKeys)

	if err := snapshot.Load(snapshotPath, kvStore); err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}

	if err := wal.Replay(walPath, kvStore); err != nil {
		t.Fatalf("failed to replay WAL: %v", err)
	}

	writeAheadLog, err := wal.Open(walPath, wal.SyncNone)
	if err != nil {
		t.Fatalf("failed to open WAL: %v", err)
	}

	kvStore.StartExpirationWorker(ctx, 10*time.Millisecond)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := NewServer(listener.Addr().String(), kvStore, writeAheadLog, snapshotPath)

	errCh := make(chan error, 1)

	go func() {
		errCh <- srv.Serve(listener)
	}()

	cleanup := func() {
		cancel()
		_ = listener.Close()
		_ = writeAheadLog.Close()

		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("server did not shut down cleanly")
		}
	}

	return listener.Addr().String(), cleanup
}

func runCommands(t *testing.T, addr string, commands ...string) []string {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	responses := make([]string, 0, len(commands))

	for _, command := range commands {
		if _, err := writer.WriteString(command + "\n"); err != nil {
			t.Fatalf("failed to write command %q: %v", command, err)
		}

		if err := writer.Flush(); err != nil {
			t.Fatalf("failed to flush command %q: %v", command, err)
		}

		response, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read response for command %q: %v", command, err)
		}

		responses = append(responses, strings.TrimSpace(response))
	}

	return responses
}

func assertResponses(t *testing.T, got []string, expected []string) {
	t.Helper()

	if len(got) != len(expected) {
		t.Fatalf("expected %d responses, got %d: %#v", len(expected), len(got), got)
	}

	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("response %d: expected %q, got %q", i, expected[i], got[i])
		}
	}
}
