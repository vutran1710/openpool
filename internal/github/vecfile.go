package github

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// WriteVecFile writes a vector as raw little-endian float32 bytes.
func WriteVecFile(path string, vec []float32) error {
	buf := make([]byte, len(vec)*4)
	for i, f := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, buf, 0600)
}

// ReadVecFile reads a vector from raw little-endian float32 bytes.
func ReadVecFile(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeVecBytes(data), nil
}

// VecRecord is a match_hash + vector pair read from the index directory.
type VecRecord struct {
	MatchHash string
	Vector    []float32
}

// ReadVecDir reads all .vec files from a directory.
// The match_hash is extracted from the filename (without extension).
func ReadVecDir(dir string) ([]VecRecord, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}
	var records []VecRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".vec") {
			continue
		}
		matchHash := strings.TrimSuffix(e.Name(), ".vec")
		vec, err := ReadVecFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		records = append(records, VecRecord{MatchHash: matchHash, Vector: vec})
	}
	return records, nil
}

func decodeVecBytes(data []byte) []float32 {
	vec := make([]float32, len(data)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return vec
}
