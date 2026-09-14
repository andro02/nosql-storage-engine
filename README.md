<h1 align="center">
  NoSQL Key-Value Storage Engine
</h1>

<p align="center">
  A from-scratch NoSQL key-value storage engine written in Go, implementing the core components of an LSM-tree-based database — write-ahead log, memtable, SSTables, compaction, caching, and a set of probabilistic data structures for approximate queries.
</p>

<div align="center">

![Go](https://img.shields.io/badge/Go-1.20-00ADD8)
![status](https://img.shields.io/badge/Status-University%20Project-yellow)

</div>

## About the Project

This project implements the storage engine of a NoSQL database, following the same design principles used by systems like Cassandra and RocksDB. Writes are first appended to a write-ahead log for durability, buffered in an in-memory memtable, and once a size threshold is reached, flushed to disk as an immutable SSTable. Reads are served from the memtable and cache first, falling back to on-disk SSTables through index and summary files, with Bloom filters used to avoid unnecessary disk lookups.

In addition to the core read/write path, the engine includes several probabilistic data structures (Bloom filter, Count-Min Sketch, HyperLogLog) commonly used in large-scale NoSQL systems for approximate membership testing, frequency estimation, and cardinality estimation, along with a token bucket for rate-limiting client requests.

## Features

- Write-ahead log (WAL) for crash recovery, with configurable segment size and low-water-mark
- In-memory memtable backed by a configurable structure (skip list or B-tree)
- SSTable persistence with separate data, index, and summary files, supporting both single-file and multi-file layouts
- Merkle tree generation over SSTables for data integrity verification
- Bloom filter for fast negative lookups before hitting disk
- LRU cache for frequently accessed keys
- Count-Min Sketch for approximate frequency counting
- HyperLogLog for approximate cardinality estimation
- Token bucket rate limiter to throttle incoming requests
- PUT / GET / DELETE operations exposed through a simple CLI, with tombstone-based deletion
- Fully configurable engine parameters (cache size, memtable threshold, WAL sizes, Bloom filter false-positive rate, B-tree/skip-list limits, rate-limiting) via `config.txt`

## Architecture

The engine follows the classic LSM-tree write path:

1. **Write-Ahead Log** — every PUT/DELETE is first appended to the WAL for durability.
2. **Memtable** — writes are buffered in memory (skip list or B-tree, configurable) until a size threshold is reached.
3. **Flush to SSTable** — once full, the memtable is flushed to disk as an immutable SSTable, made up of data, index, and summary files, plus a Bloom filter and Merkle tree for that segment.
4. **Reads** — a GET checks the memtable and cache first, then walks the on-disk SSTables (newest to oldest), using each SSTable's Bloom filter and index/summary files to avoid scanning the full data file.
5. **Rate limiting** — every incoming command is checked against a token bucket before being processed.

## Technologies

**Language:** Go 1.20

**Core structures:** Write-ahead log, skip list, B-tree, LSM-tree / SSTable, Merkle tree

**Probabilistic structures:** Bloom filter, Count-Min Sketch, HyperLogLog

**Other:** LRU cache, token bucket rate limiting, memory-mapped file I/O (`mmap-go`)

## Project Structure

```
📦 root
 ┣ 📂 config        — configuration loading (config.txt parsing)
 ┣ 📂 src
 ┃ ┣ wal.go / walEntry.go        — write-ahead log
 ┃ ┣ memtable.go                 — in-memory write buffer
 ┃ ┣ skipList.go / bTree.go / bTreeNode.go   — memtable data structures
 ┃ ┣ data.go / index.go / summary.go         — SSTable file layout
 ┃ ┣ merkle.go                   — Merkle tree for data integrity
 ┃ ┣ bloomFilter.go / hashFunctions.go       — Bloom filter
 ┃ ┣ countMinSketch.go           — approximate frequency counting
 ┃ ┣ hyperLogLog.go              — approximate cardinality estimation
 ┃ ┣ cache.go                    — LRU cache
 ┃ ┣ tokenBucket.go              — rate limiting
 ┃ ┗ put.go / get.go / delete.go — core engine operations
 ┣ main.go          — CLI entry point
 ┗ config.txt       — engine configuration
```

## Running the Project

### Prerequisites

- Go 1.20+

### Build and run

```bash
go mod tidy
go run main.go
```

The engine loads its configuration from `config.txt` in the project root and starts an interactive CLI where you can run:

```
>>> PUT <key> <value>
>>> GET <key>
>>> DELETE <key>
>>> CMS
>>> HLL
```
