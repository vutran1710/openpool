package github

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
)

// IndexRecord is a single user's filter values + similarity vector + display info, stored in the pool repo.
type IndexRecord struct {
	Filters     FilterValues `msgpack:"f"`
	Vector      []float32    `msgpack:"v"`
	DisplayName string       `msgpack:"n,omitempty"`
	About       string       `msgpack:"a,omitempty"`
	Bio         string       `msgpack:"b,omitempty"`
}

// WriteRecFile writes an index record as msgpack.
func WriteRecFile(path string, rec IndexRecord) error {
	data, err := msgpack.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encoding record: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// ReadRecFile reads an index record from msgpack.
func ReadRecFile(path string) (*IndexRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rec IndexRecord
	if err := msgpack.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("decoding record: %w", err)
	}
	return &rec, nil
}

// NamedRecord is a match_hash + index record read from the index directory.
type NamedRecord struct {
	MatchHash string
	Record    IndexRecord
}

// ReadRecDir reads all .rec files from a directory.
// The match_hash is extracted from the filename (without extension).
func ReadRecDir(dir string) ([]NamedRecord, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}
	var records []NamedRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rec") {
			continue
		}
		matchHash := strings.TrimSuffix(e.Name(), ".rec")
		rec, err := ReadRecFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		records = append(records, NamedRecord{MatchHash: matchHash, Record: *rec})
	}
	return records, nil
}
