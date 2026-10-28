package storage

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"nano-redis/internal/resp"
)

// AOF provides Append-Only File durability adhering to the write-ahead log pattern.
type AOF struct {
	file *os.File
	mu   sync.Mutex
}

// NewAOF opens or creates the log file in append mode.
func NewAOF(path string) (*AOF, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	a := &AOF{file: file}

	// Background fsync ticker running every second (corresponds to Redis 'appendfsync everysec')
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			a.mu.Lock()
			_ = a.file.Sync()
			a.mu.Unlock()
		}
	}()

	return a, nil
}

// WriteCommand formats a mutating command into a standard RESP Array and appends to the log.
func (a *AOF) WriteCommand(cmd string, args ...string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var items []resp.Value
	items = append(items, resp.BulkString(cmd))
	for _, arg := range args {
		items = append(items, resp.BulkString(arg))
	}

	val := resp.Array(items)
	bytes := val.Marshal()

	_, err := a.file.Write(bytes)
	return err
}

// Replay reads the log file from byte 0 and re-executes all persisted mutations into memory.
//
// Professor's Note:
// This is the core of Crash Recovery. In distributed systems, this is the WAL (Write Ahead Log)
// replay phase that brings state machine replicas into alignment with the committed log.
func (a *AOF) Replay(store *Store) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Seek to beginning of the file
	if _, err := a.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	reader := resp.NewReader(a.file)
	for {
		val, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		if val.Type != resp.TypeArray || len(val.Array) == 0 {
			continue
		}

		cmd := val.Array[0].Bulk
		switch cmd {
		case "SET":
			if len(val.Array) >= 3 {
				k := val.Array[1].Bulk
				v := val.Array[2].Bulk
				store.data[k] = Item{Value: v}
			}
		case "DEL":
			if len(val.Array) >= 2 {
				k := val.Array[1].Bulk
				delete(store.data, k)
			}
		}
	}

	// Seek back to end for subsequent writes
	_, err := a.file.Seek(0, io.SeekEnd)
	return err
}

func (a *AOF) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.file.Sync()
	return a.file.Close()
}
