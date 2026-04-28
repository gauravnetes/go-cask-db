package engine

import (
	"fmt"
	"os"
)

func (db *DB) Compact() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	fmt.Println("[Compactor] Starting background compaction...")

	// 1. If we have 1 or 0 files, there is nothing to compact!
	if db.nextSSTableID <= 2 {
		fmt.Println("[Compactor] Not enough files to compact. Skipping.")
		return nil
	}

	mergedData := make(map[string]Entry)

	// 2. Read all files from Oldest to Newest
	// Because we go oldest to newest, newer keys will naturally overwrite older keys in the map!
	for id := 1; id < db.nextSSTableID; id++ {
		sstPath := fmt.Sprintf("%s/%d.sst", db.dataDir, id)
		
		fileData, err := ReadSSTable(sstPath)
		if err != nil {
			continue // Skip if file doesn't exist
		}

		for key, entry := range fileData {
			// Only add it if it's NOT a tombstone. 
			// If it IS a tombstone, it deletes the key from our merged map entirely!
			if entry.Tombstone {
				delete(mergedData, key)
			} else {
				mergedData[key] = entry
			}
		}
		
		// Delete the old file from the disk now that we have its data
		os.Remove(sstPath) 
	}

	// 3. Write the fully merged, cleaned-up data to a brand new 1.sst file
	compactedPath := fmt.Sprintf("%s/1.sst", db.dataDir)
	if err := FlushToSSTable(mergedData, compactedPath); err != nil {
		return err
	}

	// 4. Reset our database file counter back to 2 
	db.nextSSTableID = 2
	
	fmt.Println("[Compactor] Compaction complete! All data squashed into 1.sst")
	return nil
}