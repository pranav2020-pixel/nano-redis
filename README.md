# nano-redis ⚡

A high-throughput, concurrent **In-Memory Key-Value Store** built from first principles in **Go**. Implements the official **Redis Serialization Protocol (RESP2)** and **Append-Only File (AOF)** persistence. Connectable via the official `redis-cli` binary.

---

## Architecture Overview

```
                      ┌─────────────────────────────────────────┐
                      │          redis-cli / Client             │
                      └────────────────────┬────────────────────┘
                                           │  TCP (Port 6379)
                                           ▼
                      ┌─────────────────────────────────────────┐
                      │             TCP Listener                │
                      └────────────────────┬────────────────────┘
                                           │  go handleClient()
                                           ▼
                      ┌─────────────────────────────────────────┐
                      │         Streaming RESP2 Parser          │
                      │  - Simple String (+)  - Error (-)       │
                      │  - Integer (:)        - Bulk String ($) │
                      │  - Array (*)                            │
                      └────────────────────┬────────────────────┘
                                           │
                         ┌─────────────────┴─────────────────┐
                         ▼                                   ▼
        ┌─────────────────────────────────┐ ┌─────────────────────────────────┐
        │   Concurrent Storage Engine     │ │      AOF Durability Engine      │
        │ - sync.RWMutex Synchronization  │ │ - Sequential Log Appends        │
        │ - Passive TTL Eviction (on GET) │ │ - Background fsync (everysec)   │
        │ - Active Probabilistic Eviction │ │ - Startup Crash Recovery Replay │
        └─────────────────────────────────┘ └─────────────────────────────────┘
```

---

## Supported Commands

| Command | Syntax | Description |
| :--- | :--- | :--- |
| `PING` | `PING [message]` | Tests connection health and latency |
| `SET` | `SET key value [EX seconds]` | Sets key with optional TTL |
| `GET` | `GET key` | Retrieves key value (with lazy expiration) |
| `DEL` | `DEL key [key ...]` | Deletes one or more keys |
| `EXISTS`| `EXISTS key [key ...]` | Counts matching active keys |
| `EXPIRE`| `EXPIRE key seconds` | Updates TTL on existing key |
| `TTL` | `TTL key` | Returns remaining lifetime in seconds |
| `KEYS` | `KEYS [pattern]` | Pattern-based key lookup (e.g. `user:*`) |
| `QUIT` | `QUIT` | Gracefully closes connection |

---

## Quickstart

### 1. Build and Run
```bash
# Clone the repository
git clone https://github.com/your-username/nano-redis.git
cd nano-redis

# Run the server on port 6379
go run main.go -port 6379 -aof database.aof
```

### 2. Connect with Official redis-cli
```bash
redis-cli -p 6379

127.0.0.1:6379> PING
PONG
127.0.0.1:6379> SET user:100 "alice" EX 60
OK
127.0.0.1:6379> GET user:100
"alice"
127.0.0.1:6379> TTL user:100
(integer) 58
```

### 3. Run Automated Tests
```bash
go test -v ./...
```

---

## Core CS Concepts Mastered

1. **Binary-Safe Wire Protocols:** Length-prefixed bulk string decoding vs null-terminated string parsing.
2. **Readers-Writer Synchronization:** Using `sync.RWMutex` to maximize concurrent read throughput while serializing write operations.
3. **Database Durability & WAL:** Writing sequential mutation logs to disk and synchronizing OS page caches via `fsync(2)` syscalls for crash recovery.
4. **Memory Reclaim Algorithms:** Hybrid TTL expiration combining zero-cost lazy passive eviction on read with active probabilistic random sampling in a background thread.
