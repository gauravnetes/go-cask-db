package engine

import (
	"encoding/binary"
	"os"
)

type WAL struct {
	file *os.File
}

// NewWAL: Append only mode
func NewWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f}, nil
}

func (w *WAL) Append(entry Entry) error {
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

	_, err := w.file.Write(payload)
	if err != nil {
		return err
	}

	return w.file.Sync()
}

func (w *WAL) Close() error {
	return w.file.Close()
}
