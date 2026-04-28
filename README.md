# go-cask-db

> A high-performance, production-inspired Key-Value storage engine written in pure Go, implementing LSM-tree and Bitcask architectures from first principles.

## Overview

**go-cask-db** is a custom database engine built to deeply understand the low-level internals of modern distributed databases like LevelDB, RocksDB, and Apache Cassandra. Rather than relying on high-level abstractions, this project implements raw disk I/O, binary encoding, and custom state management to demonstrate mastery of:

- **In-memory data structures** with concurrent access patterns (`sync.RWMutex`)
- **Write-Ahead Logging (WAL)** for crash-safe persistence
- **Sorted String Tables (SSTables)** for efficient disk-based retrieval
- **Read-path optimization** through hierarchical lookup with early-exit strategies
- **Background compaction** for garbage collection and space reclamation
- **Binary serialization** to minimize storage footprint

**Technology Stack:** Pure Go (standard library only) — no external dependencies, ORMs, or frameworks.

---

## Architecture & Design

The engine operates through four distinct, layered components:

### 1. The MemTable & WAL: In-Memory Core with Crash Safety

Every write operation follows a **write-ahead logging** pattern:

```
Client Write Request
        ↓
   [BINARY ENCODING]
        ↓
   Append to WAL File (disk)  ← Crash-safe point
        ↓
   Insert into MemTable (RAM)
        ↓
   Return ACK to client
```

**Key Design Decisions:**

- **Thread-Safe Concurrency:** All writes/reads are protected by a `sync.RWMutex`, eliminating race conditions while allowing multiple concurrent readers.
- **Write-Ahead Logging:** Every mutation is durably written to disk *before* being applied to RAM. If the process crashes, the WAL can be replayed to recover the exact state.
- **Binary Format:** Instead of JSON or text, all entries use raw binary encoding (17-byte header + key/value bytes), reducing I/O overhead and storage footprint.

**Entry Structure (Disk Format):**
```
[Timestamp (8)] [Tombstone (1)] [KeyLen (4)] [ValLen (4)] [Key...] [Value...]
     uint64           bool          uint32      uint32     bytes    bytes
```

---

### 2. SSTables: Immutable Disk Flushing

When the in-memory MemTable reaches a configurable threshold (e.g., 1,000 keys), it transitions to **compaction**:

```
MemTable Reaches Threshold
        ↓
   [FREEZE MemTable]
        ↓
   Sort All Keys (Alphabetically)
        ↓
   Write to SSTable File (e.g., 1.sst)
        ↓
   Spawn New, Fresh MemTable + WAL
        ↓
   Continue accepting writes (zero blocking)
```

**Benefits:**
- **Immutable Files:** Once written, SSTables never change, enabling safe concurrent reads.
- **Sorted Keys:** Enables binary search and early-exit optimizations during reads.
- **Zero Write Blocking:** Clients never wait; compaction happens independently.

---

### 3. The Read Engine: Hierarchical Retrieval with Early-Exit

The `Get(key)` operation follows a **strict lookup hierarchy** to minimize disk I/O:

```
Client Read Request (Get "user:123")
        ↓
   [1] Check Active MemTable (RAM) → Found? Return immediately
        ↓ (miss)
   [2] Check Newest SSTable (Disk) → Found? Return immediately
        ↓ (miss)
   [3] Check Older SSTables (Disk, oldest-to-newest)
        ↓
   [4] Return "Not Found"
```

**Critical Optimization: Early-Exit Strategy**

Since SSTables are strictly sorted alphabetically, the disk reader implements an intelligent abort mechanism:

```go
for each entry in SSTable {
    if entry.Key == target_key {
        return entry  // Found
    }
    if entry.Key > target_key {
        return "Not Found"  // Key will never appear; abort
    }
}
```

This single optimization can save **orders of magnitude** in disk reads for missing keys.

**Deletion Handling: Tombstones**

Data is never physically deleted. Instead, a "Tombstone" marker is written:
- Deletion request → Write entry with `Tombstone=true` flag
- During Read → If tombstone detected, return "Not Found" (masking old data)
- During Compaction → Permanently dropped from the database

---

### 4. The Compactor: Garbage Collection

A background worker that solves the **write amplification problem** inherent to LSM-trees:

```
Compactor Wakes (periodic schedule)
        ↓
   Read All Existing .sst Files
        ↓
   Merge & Deduplicate Keys
        ↓
   Keep: Entry with Highest Timestamp
        ↓
   Drop: All Entries with Tombstone Flag
        ↓
   Write Single Optimized .sst File
        ↓
   Delete Old .sst Files
        ↓
   Reclaimed Disk Space ✓
```

**Impact:**
- Resolves conflicts using **Unix nanosecond timestamps** (highest timestamp wins)
- Permanently removes tombstoned entries
- Reduces file count, improving future read latency
- Runs entirely in the background without blocking writes

---

## Project Structure

```
go-cask-db/
├── cmd/
│   └── server/
│       └── main.go              # Entry point & test injection
├── internal/
│   └── engine/
│       ├── db.go                # Core database coordinator
│       ├── memtable.go          # In-memory sorted map (sync.RWMutex)
│       ├── wal.go               # Write-Ahead Log (append-only disk log)
│       ├── sstable.go           # Sorted String Table (immutable disk files)
│       └── compactor.go         # Background garbage collection worker
├── data/                        # Generated directory (WAL & SSTable files)
├── go.mod                       # Go module definition
├── MakeFile                     # Build and development targets
└── README.md                    # This file
```

---

## Engineering Trade-Offs & Decisions

### 1. Raw Binary Encoding vs. JSON/Protocol Buffers

**Decision:** Raw binary with custom 17-byte header

**Rationale:**
- **Storage Efficiency:** Binary is ~5-10x smaller than JSON for typical KV pairs
- **Serialization Speed:** Direct byte manipulation faster than marshaling
- **I/O Reduction:** Fewer bytes = fewer disk cycles
- **Trade-off:** Loss of human readability (but WAL replay is still possible via custom tools)

### 2. `sync.RWMutex` for Concurrency vs. Lock-Free Data Structures

**Decision:** `sync.RWMutex` wrapping the MemTable

**Rationale:**
- **Correctness First:** Easier to reason about and debug vs. lock-free atomics
- **Fairness:** Readers don't starve writers
- **Standard Library:** No external dependencies
- **Performance:** Sufficient for single-node deployment; sharding handled at application level
- **Trade-off:** Lock contention on MemTable; mitigated by rapid SSTable flushing

### 3. Tombstones for Deletions vs. In-Place Removal

**Decision:** Tombstone markers (lazy deletion)

**Rationale:**
- **Append-Only WAL:** Deletions fit naturally as entries, not operations
- **Immutable SSTables:** Cannot rewrite existing files; tombstones defer cleanup
- **Crash Safety:** Deletion intent is durably logged immediately
- **Trade-off:** Deleted keys consume disk space until compaction; acceptable for LSM-tree design

### 4. Sorting on Flush vs. Maintaining Sorted MemTable

**Decision:** Sort on flush (unsorted MemTable, sorted on disk)

**Rationale:**
- **Write Performance:** In-memory inserts are O(1) hash-map operations
- **Flush Overhead:** One-time sort cost when MemTable reaches threshold
- **Read Performance:** Most reads hit MemTable (hot data); disk reads benefit from sorted SSTables
- **Trade-off:** SSTable read latency slightly higher, but MemTable read latency dominant

---

## How to Run

### Prerequisites

- **Go 1.18+** installed on your system
- Standard library only — no additional dependencies to install

### Building the Project

```bash
# Clone the repository
git clone https://github.com/<your-username>/go-cask-db.git
cd go-cask-db

# Build using the Makefile
make build

# Or build directly
go build -o go-cask-db ./cmd/server
```

### Running the Database Engine

```bash
# Start the engine
go run cmd/server/main.go
```

**Expected Output:**

On the first run, you will see:

1. **MemTable Operations:** Initial writes to the database
2. **WAL Creation:** `data/write-ahead.log` file generated (binary format)
3. **SSTable Flush:** When threshold is reached, `data/1.sst` file created (sorted, immutable)
4. **Fresh MemTable:** Ready for new writes
5. **Compaction Cycle:** After multiple flushes, background compactor triggers, consolidating all `.sst` files

**Example Session:**

```bash
$ go run cmd/server/main.go
2026/04/28 10:15:32 [DB] Initialized with data directory: ./data
2026/04/28 10:15:32 [WAL] Created Write-Ahead Log: ./data/write-ahead.log
2026/04/28 10:15:32 [MemTable] Active MemTable created (capacity: 1000 keys)

[Client writes 500 keys...]
2026/04/28 10:15:45 [MemTable] MemTable at 500/1000 keys

[Client writes 600 more keys...]
2026/04/28 10:15:52 [MemTable] Threshold reached (1100/1000)
2026/04/28 10:15:52 [SSTable] Flushing sorted data to: ./data/1.sst
2026/04/28 10:15:53 [MemTable] Fresh MemTable spawned
2026/04/28 10:15:53 [WAL] New WAL log: ./data/write-ahead.log

[After multiple flushes...]
2026/04/28 10:16:30 [Compactor] Starting compaction cycle
2026/04/28 10:16:30 [Compactor] Merging 3 SSTable files...
2026/04/28 10:16:35 [Compactor] Compaction complete. Old files deleted.
2026/04/28 10:16:35 [Compactor] Disk space reclaimed: 2.1 MB
```

### Interacting with the Engine via Telnet

The server exposes a **text-based TCP protocol** accessible via telnet:

```bash
# Open a telnet connection to the server (default: localhost:8080)
telnet localhost 8080

# You should see:
# Connected to localhost.
# Connected to go-cask-db. Type a command (e.g., SET key value, GET key):
```

#### Available Commands

| Command | Syntax | Description | Example |
|---------|--------|-------------|---------|
| **SET** | `SET <key> <value>` | Store a key-value pair | `SET user:123 "John Doe"` |
| **GET** | `GET <key>` | Retrieve a value by key | `GET user:123` |
| **COMPACT** | `COMPACT` | Trigger compaction cycle | `COMPACT` |
| **PING** | `PING` | Health check | `PING` |
| **EXIT/QUIT** | `EXIT` or `QUIT` | Close connection | `EXIT` |

#### Example Session

```
Connected to go-cask-db. Type a command (e.g., SET key value, GET key):
SET target "company"
OK
SET company "Google"
OK
SET position "Software Engineering Intern"
OK
GET target
company
GET company
Google
GET position
Software Engineering Intern
COMPACT
OK: Compaction complete
PING
PONG
EXIT
Goodbye!
```

#### Under the Hood

- **Multi-client Support:** Each client connection spawns a dedicated goroutine, allowing hundreds of concurrent users without blocking
- **Binary Safety:** Internally, values are stored as raw bytes; the telnet interface handles string conversion
- **Write-Through:** Every `SET` command is immediately written to the WAL before the MemTable is updated (durability guaranteed)
- **Read Optimization:** `GET` commands follow the hierarchical lookup (MemTable → Newest SSTable → Older SSTables)

### Inspecting Database Files

After running, inspect the `data/` directory:

```bash
ls -lh data/

# Output:
# -rw-r--r--  1 user  staff   512K Apr 28 10:15 write-ahead.log
# -rw-r--r--  1 user  staff   1.2M Apr 28 10:15 1.sst
# -rw-r--r--  1 user  staff   1.1M Apr 28 10:16 2.sst
```

**Binary files are not human-readable.** To inspect contents, use the debugging tools in `cmd/server/main.go` to replay logs or decode SSTables.

---

## Key Insights & Learning Outcomes

This project demonstrates:

✅ **Low-level database internals** — How LevelDB, RocksDB, and Cassandra manage data at scale

✅ **Concurrency patterns** — Thread-safe access without sacrificing performance

✅ **I/O optimization** — Strategic caching (MemTable) + early-exit disk reads

✅ **Trade-off analysis** — Making intentional architectural decisions with explicit trade-offs

✅ **Production patterns** — Write-ahead logging, compaction, tombstones (real techniques used in production systems)

✅ **Go idioms** — Pure stdlib, idiomatic error handling, clean interfaces

---

## Future Enhancements

- [ ] Bloom filters for faster SSTable searches (reduce disk I/O)
- [ ] Configurable compaction strategies (leveled vs. tiered compaction)
- [ ] Crash recovery from corrupted WAL entries
- [ ] Index blocks within SSTables (sparse indexing)
- [ ] Multi-threaded compaction for large datasets
- [ ] Network layer (gRPC or custom TCP for remote clients)

---

## References & Inspiration

- **LevelDB:** https://github.com/google/leveldb
- **RocksDB:** https://github.com/facebook/rocksdb
- **Designing Data-Intensive Applications** by Martin Kleppmann (Chapter 3: Storage and Retrieval)
- **The Bitcask Paper:** Riak's key-value store architecture
- **Go Concurrency Patterns:** https://go.dev/blog/pipelines

---

## License

MIT License — See LICENSE file for details.

---

**Built as a flagship Database Engineering internship portfolio project.**
