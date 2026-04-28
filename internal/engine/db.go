package engine

import (
	"fmt"
	"sync"
	"time"
	"os"
	"strings"
	"strconv"
)

type Entry struct {
	Key       string
	Value     []byte
	Timestamp int64
	Tombstone bool // true for deletion marker
}

type DB struct {
	mu            sync.RWMutex
	memtable      *MemTable
	wal           *WAL
	nextSSTableID int
	dataDir       string
}

func NewDB(dataDir string) (*DB, error) {
	walPath := fmt.Sprintf("%s/wal.log", dataDir)
	wal, err := NewWAL(walPath)

	if err != nil {
		return nil, err
	}
	nextID := 1 
	files, err := os.ReadDir(dataDir)
	if err == nil {
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".sst") {
				idStr := strings.TrimSuffix(file.Name(), ".sst")
				id, err := strconv.Atoi(idStr)

				if err == nil && id >= nextID {
					nextID = id + 1 
				}
			}
		}
	}
	return &DB{
		memtable:      NewMemTable(),
		wal:           wal,
		nextSSTableID: nextID,
		dataDir:       dataDir,
	}, nil
}


func (db *DB) Put(key string, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	entry := Entry {
		Key:	key, 
		Value:	value, 
		Timestamp: time.Now().UnixNano(), 
		Tombstone:	false, 
	}

	if err := db.wal.Append(entry); err != nil {
		return err
	}

	db.memtable.Put(entry)

	if len(db.memtable.data) >= 1000 {
		return db.flushMemTable()
	}

	return nil
}


func (db *DB) flushMemTable() error {
	sstPath := fmt.Sprintf("%s/%d.sst", db.dataDir, db.nextSSTableID)
	db.nextSSTableID++

	if err := FlushToSSTable(db.memtable.data, sstPath); err != nil {
		return err
	}

	db.wal.Close()
	walPath := fmt.Sprintf("%s/wal.log", db.dataDir)

	newWal, err := NewWAL(walPath)
	if err != nil {
		return err
	}

	db.wal = newWal
	db.memtable = NewMemTable()

	fmt.Printf("Successfully flushed MemTable to %s\n", sstPath)
	return nil
}

func (db *DB) Get(key string) ([]byte, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if entry, exists := db.memtable.Get(key); exists {
		if entry.Tombstone {
			return nil, fmt.Errorf("Key not found (deleted)")
		}
		return entry.Value, nil 
	}

	for id := db.nextSSTableID - 1; id >= 1; id-- {
		sstPath := fmt.Sprintf("%s/%d.sst", db.dataDir, id) 

		entry, found, err := SearchSSTable(sstPath, key)
		if err != nil {
			return nil, err 
		}

		if found {
			if entry.Tombstone {
				return nil, fmt.Errorf("Key not found(deleted)")
			}
			return entry.Value, nil 
		}
	}

	return nil, fmt.Errorf("Key not found")
}