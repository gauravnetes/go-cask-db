package engine

import (
	"encoding/binary"
	"io"
	"os"
	"sort"
)

// takes raw map data, sorts it, writes an immutable file
func FlushToSSTable(data map[string]Entry, path string) error {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer f.Close()

	for _, k := range keys {
		entry := data[k]

		keyBytes := []byte(entry.Key)

		// 8 (time) + 1 (bool) + 4 (key len) + 4 (val len)
		headerSize := 17
		payload := make([]byte, headerSize+len(keyBytes)+len(entry.Value))

		binary.LittleEndian.PutUint64(payload[0:8], uint64(entry.Timestamp))
		if entry.Tombstone {
			payload[8] = 1
		} else {
			payload[8] = 0
		}

		binary.LittleEndian.PutUint32(payload[9:13], uint32(len(keyBytes)))
		binary.LittleEndian.PutUint32(payload[13:17], uint32(len(entry.Value)))

		copy(payload[17:], keyBytes)
		copy(payload[17+len(keyBytes):], entry.Value)

		if _, err := f.Write(payload); err != nil {
			return err
		}
	}

	// Force the OS to flush the file buffer to physical disk
	return f.Sync()
}

func SearchSSTable(filepath string, searchKey string) (Entry, bool, error) {
	f, err := os.Open(filepath)

	if err != nil {
		return Entry{}, false, err
	}

	defer f.Close()

	for {
		header := make([]byte, 17)
		_, err := io.ReadFull(f, header)
		if err == io.EOF {
			break 
		}

		if err != nil {
			return Entry{}, false, err 
		}

		timestamp := int64(binary.LittleEndian.Uint64(header[0:8]))
		tombstone := header[8] == 1 
		keyLen := binary.LittleEndian.Uint32(header[9:13])
		valLen := binary.LittleEndian.Uint32(header[13:17])

		keyBytes := make([]byte, keyLen) 
		if _, err := io.ReadFull(f, keyBytes); err != nil {
			return Entry{}, false, err 
		}

		currentKey := string(keyBytes)

		// early exit (files strictly sorted) 
		if currentKey > searchKey {
			return Entry{}, false, nil 
		}

		if currentKey == searchKey {
			valBytes := make([]byte, valLen)
			if _, err := io.ReadFull(f, valBytes); err != nil {
				return Entry{}, false, err 
			}

			return Entry {
				Key:	searchKey, 
				Value:	valBytes, 
				Timestamp:	timestamp,  
				Tombstone: tombstone,
			}, true, nil 
		}

		f.Seek(int64(valLen), io.SeekCurrent)
	}
	return Entry{}, false, nil
}


func ReadSSTable(filepath string) (map[string] Entry, error) {
	data := make(map[string]Entry) 

	f, err := os.Open(filepath)

	if err != nil {
		return data, err 
	}

	defer f.Close()

	for {
		header := make([]byte, 17)
		_, err := io.ReadFull(f, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return data, err
		}

		timestamp := int64(binary.LittleEndian.Uint64(header[0:8]))
		tombstone := header[8] == 1
		keyLen := binary.LittleEndian.Uint32(header[9:13])
		valLen := binary.LittleEndian.Uint32(header[13:17])

		keyBytes := make([]byte, keyLen)
		io.ReadFull(f, keyBytes)
		
		valBytes := make([]byte, valLen)
		io.ReadFull(f, valBytes)

		data[string(keyBytes)] = Entry{
			Key:       string(keyBytes),
			Value:     valBytes,
			Timestamp: timestamp,
			Tombstone: tombstone,
		}
	}
	return data, nil 
}