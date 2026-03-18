package suggestions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vmihailenco/msgpack/v5"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

// Record holds a user's filter values + similarity vector.
type Record struct {
	MatchHash string          `msgpack:"m"`
	Filters   gh.FilterValues `msgpack:"f"`
	Vector    []float32       `msgpack:"v"`
}

// Pack is the complete pool suggestion index.
type Pack struct {
	Records []Record `msgpack:"records"`
}

// Load loads a suggestions pack from disk.
func Load(path string) (*Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Pack{}, nil
		}
		return nil, fmt.Errorf("reading pack: %w", err)
	}
	var pack Pack
	if err := msgpack.Unmarshal(data, &pack); err != nil {
		return nil, fmt.Errorf("decoding pack: %w", err)
	}
	return &pack, nil
}

// Save writes the suggestions pack to disk.
func (p *Pack) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	data, err := msgpack.Marshal(p)
	if err != nil {
		return fmt.Errorf("encoding pack: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// Upsert adds or updates a record by match_hash.
func (p *Pack) Upsert(r Record) {
	for i, existing := range p.Records {
		if existing.MatchHash == r.MatchHash {
			p.Records[i] = r
			return
		}
	}
	p.Records = append(p.Records, r)
}

// Find returns a record by match_hash, or nil if not found.
func (p *Pack) Find(matchHash string) *Record {
	for i := range p.Records {
		if p.Records[i].MatchHash == matchHash {
			return &p.Records[i]
		}
	}
	return nil
}

// SyncFromRecDir reads .rec files and upserts new records (with filters + vectors).
// Returns number of new records added.
func (p *Pack) SyncFromRecDir(dir string) (int, error) {
	namedRecords, err := gh.ReadRecDir(dir)
	if err != nil {
		return 0, fmt.Errorf("reading rec dir: %w", err)
	}

	added := 0
	for _, nr := range namedRecords {
		if p.Find(nr.MatchHash) != nil {
			continue
		}
		p.Records = append(p.Records, Record{
			MatchHash: nr.MatchHash,
			Filters:   nr.Record.Filters,
			Vector:    nr.Record.Vector,
		})
		added++
	}
	return added, nil
}
