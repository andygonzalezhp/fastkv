# FastKV

FastKV is a high-performance key-value store written in Go.

It exposes a simple TCP protocol and supports concurrent clients using goroutines and a thread-safe in-memory storage engine.

## Features

Current:

- TCP server
- Concurrent client handling
- Thread-safe in-memory key-value store
- Basic text protocol
- Commands: `PING`, `SET`, `GET`, `DEL`, `QUIT`

Planned:

- TTL expiration
- Write-ahead log persistence
- Snapshotting
- Benchmarks
- LRU eviction
- Replication
- Sharding

## Commands

### PING

```txt
PING