package suggestions

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"math/rand"
	"sort"

	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/pooldb"
)

// Suggestion is a ranked match_hash with similarity score.
type Suggestion struct {
	MatchHash string
	Score     float64
}

// Tier boundaries: top 5%, 5-20%, 20-50%, 50-100%
var tierBounds = []float64{0.05, 0.20, 0.50, 1.00}

// RankSuggestions filters by schema, excludes self + seen, then produces
// a tier-ordered shuffled list: top 5% first (shuffled), then 5-20%, then 20-50%, then 50-100%.
func RankSuggestions(schema *gh.PoolSchema, me Record, records []Record, seen map[string]bool, limit int) []Suggestion {
	// Score all candidates
	var scored []Suggestion
	for _, r := range records {
		if r.MatchHash == me.MatchHash {
			continue
		}
		if seen != nil && seen[r.MatchHash] {
			continue
		}
		if schema != nil && !gh.IsMatch(schema, me.Filters, r.Filters) {
			continue
		}
		scored = append(scored, Suggestion{
			MatchHash: r.MatchHash,
			Score:     cosine(me.Vector, r.Vector),
		})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Tier-based shuffling: 5%, 20%, 50%, 100%
	var result []Suggestion
	n := len(scored)
	prev := 0
	for _, bound := range tierBounds {
		end := int(float64(n) * bound)
		if end > n {
			end = n
		}
		if end <= prev {
			continue
		}
		tier := make([]Suggestion, end-prev)
		copy(tier, scored[prev:end])
		rand.Shuffle(len(tier), func(i, j int) {
			tier[i], tier[j] = tier[j], tier[i]
		})
		result = append(result, tier...)
		prev = end
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// ProfileToRecord converts a pooldb.Profile to a suggestions Record.
func ProfileToRecord(p pooldb.Profile) Record {
	var filters gh.FilterValues
	json.Unmarshal([]byte(p.Filters), &filters)
	return Record{
		MatchHash:   p.MatchHash,
		Filters:     filters,
		Vector:      decodeFloat32s(p.Vector),
		DisplayName: p.DisplayName,
		About:       p.About,
		Bio:         p.Bio,
	}
}

// decodeFloat32s decodes little-endian bytes to a slice of float32 values.
func decodeFloat32s(b []byte) []float32 {
	if len(b) == 0 {
		return nil
	}
	fs := make([]float32, len(b)/4)
	for i := range fs {
		fs[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return fs
}

func cosine(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := 0; i < n; i++ {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
