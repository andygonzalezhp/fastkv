# FastKV

![CI](https://github.com/andygonzalezhp/fastkv/actions/workflows/ci.yml/badge.svg)

**FastKV** is a high-performance key-value store written in Go.

It exposes a lightweight TCP protocol and implements core storage-system features including concurrent client handling, TTL expiration, write-ahead log persistence, snapshotting, LRU eviction, configurable durability policies, benchmarking, Docker support, and CI-backed automated testing.

FastKV is designed as a Redis-inspired systems project focused on the engineering tradeoffs behind real infrastructure software.

---

## Highlights

- Concurrent TCP server written in Go
- Thread-safe in-memory storage engine
- TTL expiration with background cleanup
- Write-ahead log persistence
- Snapshot persistence with WAL truncation
- Configurable WAL sync policy for durability/performance tradeoffs
- LRU eviction with configurable max key capacity
- End-to-end TCP integration tests
- TCP benchmark client with 100k+ ops/sec local results
- Dockerized deployment
- GitHub Actions CI

---

## Why FastKV Exists

FastKV is not intended to replace Redis.

The goal is to build a practical storage engine from scratch and expose the same kinds of tradeoffs that real infrastructure systems deal with:

- How should data survive restarts?
- When should writes be flushed to disk?
- How should expired keys be removed?
- What happens when memory capacity is reached?
- How do benchmarks change when durability guarantees change?
- How can a simple TCP service be tested end-to-end?

---

## Feature Overview

| Area | Support |
|---|---|
| TCP server | Yes |
| Concurrent clients | Yes |
| In-memory key-value storage | Yes |
| Thread-safe reads/writes | Yes |
| TTL expiration | Yes |
| Background expiration worker | Yes |
| Write-ahead log | Yes |
| Snapshot persistence | Yes |
| WAL truncation | Yes |
| Configurable WAL sync policy | Yes |
| LRU eviction | Yes |
| Configurable max key capacity | Yes |
| Stats command | Yes |
| TCP benchmark client | Yes |
| Unit tests | Yes |
| TCP integration tests | Yes |
| Docker | Yes |
| GitHub Actions CI | Yes |

---

## Supported Commands

| Command | Description |
|---|---|
| `PING` | Health check command |
| `SET <key> <value>` | Stores a key-value pair |
| `GET <key>` | Retrieves a value by key |
| `DEL <key>` | Deletes a key |
| `EXPIRE <key> <seconds>` | Sets a TTL on a key |
| `TTL <key>` | Returns remaining TTL in seconds |
| `DBSIZE` | Returns the number of keys currently stored |
| `STATS` | Returns key count, max keys, eviction policy, and eviction count |
| `SAVE` | Writes a snapshot and truncates the WAL |
| `QUIT` / `EXIT` | Closes the client connection |

---

## Architecture

FastKV is organized into small internal packages for networking, storage, persistence, snapshots, and benchmarking.

```txt
fastkv/
├── cmd/
│   ├── fastkv/              # Server entrypoint
│   └── bench/               # TCP benchmark client
│
├── internal/
│   ├── server/              # TCP server, command parsing, connection handling
│   ├── store/               # Thread-safe in-memory storage engine + LRU
│   ├── wal/                 # Write-ahead log persistence
│   └── snapshot/            # Snapshot save/load logic
│
├── data/                    # Runtime WAL and snapshot files
├── .github/workflows/       # GitHub Actions CI
├── Dockerfile
├── Makefile
└── README.md
```

### Request Flow

```txt
┌─────────────────────────────────────────────────────────────────────┐
│                              TCP Client                             │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
                                │  PING / SET / GET / DEL / EXPIRE
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                           FastKV Server                             │
│                                                                     │
│   ┌───────────────────────┐        ┌────────────────────────────┐   │
│   │   Connection Handler  │───────▶│      Command Parser        │   │
│   └───────────────────────┘        └────────────┬───────────────┘   │
│                                                  │                   │
└──────────────────────────────────────────────────┼───────────────────┘
                                                   │
                    ┌──────────────────────────────┼──────────────────────────────┐
                    │                              │                              │
                    ▼                              ▼                              ▼
        ┌──────────────────────┐       ┌──────────────────────┐       ┌──────────────────────┐
        │     Read Path        │       │     Write Path       │       │    Snapshot Path     │
        │                      │       │                      │       │                      │
        │  GET / TTL / STATS   │       │ SET / DEL / EXPIRE   │       │        SAVE          │
        └──────────┬───────────┘       └──────────┬───────────┘       └──────────┬───────────┘
                   │                              │                              │
                   ▼                              ▼                              ▼
        ┌──────────────────────┐       ┌──────────────────────┐       ┌──────────────────────┐
        │  In-Memory Store     │       │  Write-Ahead Log     │       │   Snapshot File      │
        │                      │       │                      │       │                      │
        │ map + RWMutex + LRU  │       │  append mutation     │       │ compact store state  │
        └──────────────────────┘       └──────────┬───────────┘       └──────────┬───────────┘
                                                   │                              │
                                                   ▼                              ▼
                                        ┌──────────────────────┐       ┌──────────────────────┐
                                        │  In-Memory Store     │       │    WAL Truncate      │
                                        │                      │       │                      │
                                        │ apply mutation       │       │ clear old log        │
                                        └──────────────────────┘       └──────────────────────┘
```

### Recovery Flow

```txt
┌─────────────────────┐
│   Start FastKV      │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Load snapshot      │
│  data/fastkv.snapshot
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Replay WAL         │
│  data/fastkv.wal    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Remove expired keys │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Start TCP server    │
└─────────────────────┘
```

---

## Persistence Model

FastKV uses a write-ahead log for durability.

For write commands such as:

```txt
SET name andy
DEL name
EXPIRE name 10
```

FastKV persists the mutation before applying it to the in-memory store.

The basic write path is:

```txt
client command
    ↓
append mutation to WAL
    ↓
optionally fsync depending on sync policy
    ↓
apply mutation to in-memory store
    ↓
return response to client
```

This allows FastKV to rebuild its state after a restart.

---

## Snapshotting

The `SAVE` command writes the current in-memory state to a snapshot file and truncates the WAL.

```txt
SAVE
OK
```

This prevents the WAL from growing forever.

Runtime files:

```txt
data/fastkv.wal
data/fastkv.snapshot
```

These files are ignored by Git because they are local runtime data.

---

## TTL Expiration

FastKV supports key expiration:

```txt
SET session abc123
EXPIRE session 10
TTL session
```

Example response:

```txt
8
```

After expiration:

```txt
GET session
(nil)

TTL session
-2
```

TTL values are persisted as absolute expiration timestamps.

For example:

```txt
EXPIRE temp 10
```

is internally persisted as:

```txt
EXPIREAT temp <unix-nanoseconds>
```

This prevents TTLs from incorrectly resetting after a restart.

---

## LRU Eviction

FastKV supports bounded memory mode using LRU eviction.

Start the server with a max key limit:

```bash
go run ./cmd/fastkv -max-keys=3
```

Example:

```txt
SET a 1
SET b 2
SET c 3
GET a
SET d 4
GET b
GET a
GET c
GET d
```

Expected behavior:

```txt
OK
OK
OK
1
OK
(nil)
1
3
4
```

Why?

```txt
a, b, c are inserted
GET a makes a recently used
SET d exceeds capacity
b is least recently used
b gets evicted
```

Stats can be inspected with:

```txt
STATS
```

Example:

```txt
keys=3 max_keys=3 eviction_policy=lru evictions=1
```

---

## WAL Sync Policy

FastKV supports configurable WAL sync behavior.

### Stronger durability

```bash
go run ./cmd/fastkv -sync-policy=always
```

### Higher write throughput

```bash
go run ./cmd/fastkv -sync-policy=none
```

| Policy | Behavior | Tradeoff |
|---|---|---|
| `always` | Calls `fsync` after every write | Stronger durability, lower write throughput |
| `none` | Writes to the WAL without forcing immediate disk sync | Higher throughput, weaker crash durability |

This exposes a real storage-system tradeoff between durability and performance.

---

## Running Locally

Start the server:

```bash
make run
```

By default, FastKV listens on port `6380`.

Connect with netcat:

```bash
nc localhost 6380
```

Example session:

```txt
PING
PONG
SET name andy
OK
GET name
andy
EXPIRE name 10
1
TTL name
8
STATS
keys=1 max_keys=unlimited eviction_policy=none evictions=0
DEL name
1
GET name
(nil)
QUIT
BYE
```

---

## Running with LRU Eviction

Start FastKV with a max key limit:

```bash
make run-lru
```

Or manually:

```bash
go run ./cmd/fastkv -max-keys=3
```

For a clean LRU demo, remove old local runtime data first:

```bash
rm -f data/fastkv.wal data/fastkv.snapshot
make run-lru
```

---

## Running with Faster Write Throughput

Run FastKV with WAL sync disabled:

```bash
make run-fast
```

Equivalent command:

```bash
go run ./cmd/fastkv -sync-policy=none
```

This mode is useful for benchmarking write-heavy workloads.

---

## Running with Docker

Build the Docker image:

```bash
make docker-build
```

Run the container:

```bash
make docker-run
```

Connect:

```bash
nc localhost 6380
```

---

## Benchmarks

FastKV includes a TCP benchmark client under `cmd/bench`.

Benchmarks were run locally against the TCP server using:

```txt
50 concurrent clients
1,000 operations per client
50,000 total operations
1,000 key keyspace
```

### Benchmark Results

| Workload | WAL Sync Policy | Throughput | Avg Latency | Errors |
|---|---:|---:|---:|---:|
| GET | `always` | 108,699 ops/sec | 455.603µs | 0 |
| SET | `always` | 251 ops/sec | 198.639ms | 0 |
| MIXED | `always` | 1,254 ops/sec | 38.282ms | 0 |
| SET | `none` | 103,266 ops/sec | 473.546µs | 0 |
| MIXED | `none` | 114,695 ops/sec | 431.944µs | 0 |

The benchmark results show the cost of synchronous durability.

With `sync-policy=always`, every write is flushed to disk before returning. This improves durability but significantly reduces write throughput.

With `sync-policy=none`, FastKV relies on the operating system page cache, which significantly improves write throughput at the cost of weaker crash durability.

---

## Running Benchmarks

Start the server first:

```bash
make run
```

Then run benchmarks in another terminal:

```bash
make bench-get
make bench-set
make bench-mixed
```

For high-throughput write benchmarks, start the server with:

```bash
make run-fast
```

Then run:

```bash
make bench-set
make bench-mixed
```

---

## Testing

Run all tests:

```bash
make test
```

Run formatting:

```bash
make fmt
```

Build all commands:

```bash
go build ./...
```

Run the full local check:

```bash
make fmt
go test ./...
go build ./...
docker build -t fastkv .
```

---

## Test Coverage

FastKV includes both unit tests and TCP integration tests.

Current coverage includes:

- Store `SET`, `GET`, and `DEL`
- Missing key behavior
- TTL expiration
- Background expiration worker
- WAL replay
- WAL replay with expired keys
- Snapshot save/load
- Snapshot loading with expired keys
- TCP command behavior
- TCP TTL behavior
- TCP LRU behavior
- TCP WAL recovery
- TCP snapshot recovery

---

## Graceful Shutdown

FastKV handles shutdown signals such as `Ctrl+C`.

On shutdown, the server:

1. Stops accepting new TCP connections
2. Cancels background workers
3. Closes the listener
4. Syncs and closes the WAL
5. Exits cleanly

Example:

```txt
shutdown signal received
FastKV shut down cleanly
```

---

## CI

FastKV uses GitHub Actions to automatically run checks on every push and pull request to `main`.

CI verifies:

- Go formatting
- Unit tests
- TCP integration tests
- Go build
- Docker image build

Workflow file:

```txt
.github/workflows/ci.yml
```

---

## Makefile Commands

| Command | Description |
|---|---|
| `make run` | Runs the FastKV server |
| `make run-fast` | Runs the server with `sync-policy=none` |
| `make run-lru` | Runs the server with `max-keys=3` |
| `make build` | Builds the FastKV binary |
| `make test` | Runs all Go tests |
| `make fmt` | Formats Go code |
| `make clean` | Removes build artifacts |
| `make docker-build` | Builds the Docker image |
| `make docker-run` | Runs FastKV in Docker |
| `make bench-get` | Runs GET benchmark |
| `make bench-set` | Runs SET benchmark |
| `make bench-mixed` | Runs mixed workload benchmark |

---

## Configuration Flags

| Flag | Default | Description |
|---|---:|---|
| `-addr` | `:6380` | TCP address for the server |
| `-wal` | `data/fastkv.wal` | Path to the write-ahead log file |
| `-snapshot` | `data/fastkv.snapshot` | Path to the snapshot file |
| `-sync-policy` | `always` | WAL sync policy: `always` or `none` |
| `-max-keys` | `0` | Maximum number of keys before LRU eviction. `0` means unlimited |

Example:

```bash
go run ./cmd/fastkv \
  -addr=:6380 \
  -wal=data/fastkv.wal \
  -snapshot=data/fastkv.snapshot \
  -sync-policy=always \
  -max-keys=10000
```

---

## Design Decisions

### TCP instead of HTTP

FastKV uses a raw TCP protocol instead of HTTP to stay closer to lightweight command protocols used by in-memory data stores.

### RWMutex for the storage engine

The store uses `sync.RWMutex` so multiple readers can access the map concurrently while writes remain exclusive.

### WAL before memory mutation

Write commands are appended to the WAL before mutating the in-memory store. This follows the write-ahead logging pattern used by many storage systems.

### Configurable durability

FastKV supports both safer synchronous writes and faster asynchronous OS-buffered writes through the WAL sync policy.

This makes the durability/performance tradeoff visible and measurable.

### Absolute expiration timestamps

TTL commands are persisted as absolute expiration timestamps instead of relative durations.

This prevents keys from receiving a fresh TTL after restart.

### Snapshot + WAL recovery

Snapshots provide a compact persisted state. WAL replay handles changes that happened after the most recent snapshot.

This avoids replaying the entire mutation history on every startup.

### LRU eviction

When `max-keys` is enabled, FastKV tracks key usage and evicts the least recently used key when capacity is exceeded.

This turns FastKV from an unbounded in-memory map into a bounded cache-like storage system.

### Graceful shutdown

The server handles interrupt signals and closes resources cleanly instead of relying on abrupt process termination.

---

## Example Recovery Flow

Start server:

```bash
make run
```

Set data:

```txt
SET name andy
SET language go
SAVE
QUIT
```

Restart server:

```bash
make run
```

Reconnect:

```bash
nc localhost 6380
```

Verify data survived restart:

```txt
GET name
andy
GET language
go
DBSIZE
2
```

---

## What FastKV Demonstrates

FastKV demonstrates core backend and systems engineering concepts:

- TCP networking
- Concurrent server design
- Goroutines
- Synchronization primitives
- Thread-safe in-memory storage
- TTL expiration
- Background workers
- Write-ahead logging
- Snapshot persistence
- WAL compaction
- LRU eviction
- Benchmarking
- Durability/performance tradeoffs
- Dockerized deployment
- CI-backed development
- End-to-end integration testing

---

## Current Limitations

FastKV is currently a single-node storage engine.

It does not currently implement:

- Distributed consensus
- Replication
- Sharding
- Authentication
- RESP compatibility
- Pipelined command execution
- Network-level encryption

These are intentional boundaries for the current version.

The project focuses on building and understanding the core internals of a durable, concurrent, in-memory key-value store.

---

## Roadmap

Potential future improvements:

- RESP-compatible protocol parser
- Pipelined command support
- Configurable asynchronous WAL sync interval
- Primary-replica replication
- Raft-based consensus and leader election
- Sharding / cluster mode
- Metrics endpoint
- Memory usage accounting
- Configurable eviction policies
- Authentication
- TLS support

---

## Summary

FastKV is a from-scratch key-value store built to explore the internals of backend infrastructure systems.

It combines:

```txt
networking
+ concurrency
+ storage
+ persistence
+ eviction
+ benchmarking
+ testing
+ deployment
```

into a compact but practical systems project.

The result is a Redis-inspired single-node storage engine that exposes the same kinds of engineering tradeoffs real infrastructure software has to make.