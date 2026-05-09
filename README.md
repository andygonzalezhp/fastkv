# FastKV

![CI](https://github.com/andygonzalezhp/fastkv/actions/workflows/ci.yml/badge.svg)

FastKV is a high-performance key-value store written in Go.

It exposes a simple TCP protocol and supports concurrent clients using goroutines, a thread-safe in-memory storage engine, TTL expiration, write-ahead log persistence, snapshotting, and configurable durability/performance tradeoffs.

FastKV is designed as a systems project inspired by Redis-style in-memory storage engines.

---

## Features

- TCP server
- Concurrent client handling with goroutines
- Thread-safe in-memory key-value store using `sync.RWMutex`
- Basic text protocol
- TTL expiration support
- Background expiration worker
- Write-ahead log persistence
- Snapshot persistence
- WAL truncation through manual snapshotting
- Configurable WAL sync policy
- TCP benchmark client
- Unit tests for store, WAL, and snapshot persistence
- Docker support
- GitHub Actions CI

---

## Supported Commands

| Command | Description |
|---|---|
| `PING` | Health check command |
| `SET <key> <value>` | Stores a key-value pair |
| `GET <key>` | Retrieves a value by key |
| `DEL <key>` | Deletes a key |
| `EXPIRE <key> <seconds>` | Sets a TTL on a key |
| `TTL <key>` | Returns remaining TTL |
| `DBSIZE` | Returns number of keys in memory |
| `SAVE` | Writes a snapshot and truncates the WAL |
| `QUIT` / `EXIT` | Closes the client connection |

---

## Architecture

FastKV is organized into separate packages for networking, storage, persistence, and snapshotting.

```txt
fastkv/
├── cmd/
│   ├── fastkv/          # FastKV server entrypoint
│   └── bench/           # TCP benchmark client
├── internal/
│   ├── server/          # TCP server and command handling
│   ├── store/           # Thread-safe in-memory storage engine
│   ├── wal/             # Write-ahead log persistence
│   └── snapshot/        # Snapshot save/load support
├── data/                # Runtime WAL/snapshot files
├── Dockerfile
├── Makefile
└── README.md
```

High-level request flow:

```txt
TCP client
   |
   v
FastKV TCP server
   |
   v
Command parser
   |
   +---- read command ----> in-memory store
   |
   +---- write command ---> WAL append ---> in-memory store
   |
   +---- SAVE command ----> snapshot file ---> WAL truncate
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

FastKV appends the mutation to the WAL before applying it to the in-memory store.

On restart:

```txt
1. Load snapshot from disk
2. Replay WAL commands
3. Remove expired keys
4. Start accepting client connections
```

This allows FastKV to recover its state after a process restart.

---

## Snapshotting

The `SAVE` command writes the current in-memory state to a snapshot file and truncates the WAL.

```txt
SAVE
OK
```

This prevents the WAL from growing forever and reduces startup replay time.

Runtime files:

```txt
data/fastkv.wal
data/fastkv.snapshot
```

These files are ignored by Git because they are local runtime data.

---

## WAL Sync Policy

FastKV supports configurable WAL sync behavior.

```bash
go run ./cmd/fastkv -sync-policy=always
```

or:

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
DEL name
1
GET name
(nil)
QUIT
BYE
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

The benchmark results show the cost of synchronous durability. With `sync-policy=always`, every write is flushed to disk before returning. With `sync-policy=none`, FastKV relies on the operating system page cache, significantly improving write throughput.

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

## CI

FastKV uses GitHub Actions to automatically run checks on every push and pull request to `main`.

CI currently verifies:

- Go formatting
- Unit tests
- Go build
- Docker image build

Workflow file:

```txt
.github/workflows/ci.yml
```

---

## Design Decisions

### TCP instead of HTTP

FastKV uses a raw TCP protocol instead of HTTP to stay closer to how real in-memory data stores expose lightweight command protocols.

### RWMutex for the storage engine

The store uses `sync.RWMutex` so multiple readers can access the map concurrently while writes remain exclusive.

### WAL before memory mutation

Write commands are appended to the WAL before mutating the in-memory store. This is the standard write-ahead logging pattern used by many storage systems.

### Absolute expiration timestamps

TTL commands are persisted as absolute expiration timestamps instead of relative durations.

For example:

```txt
EXPIRE temp 10
```

is persisted internally as:

```txt
EXPIREAT temp <unix-nanoseconds>
```

This prevents TTLs from incorrectly resetting after a restart.

### Snapshot + WAL recovery

Snapshots provide a compact persisted state. WAL replay handles changes that happened after the most recent snapshot.

This avoids replaying the entire mutation history on every startup.

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

## Makefile Commands

| Command | Description |
|---|---|
| `make run` | Runs the FastKV server |
| `make run-fast` | Runs the server with `sync-policy=none` |
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

## Roadmap

Planned improvements:

- LRU eviction policy
- Configurable max memory
- RESP-compatible protocol parser
- Pipelined command support
- Asynchronous WAL sync interval
- Replication
- Sharding
- Metrics endpoint
- Graceful shutdown
- Integration tests for TCP server behavior

---

## Why This Project Matters

FastKV demonstrates core backend and systems engineering concepts:

- TCP networking
- Concurrent server design
- Synchronization primitives
- In-memory storage engines
- TTL expiration
- Write-ahead logging
- Snapshot persistence
- Benchmarking
- Durability/performance tradeoffs
- Dockerized deployment
- CI-backed development workflow

The goal is not to replace Redis, but to build a practical, understandable storage engine that exposes the same kinds of tradeoffs real infrastructure systems face.