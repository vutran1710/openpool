package suggestions

import (
	"math"
	"math/rand"
	"sort"

	gh "github.com/vutran1710/dating-dev/internal/github"
)

// Suggestion is a ranked match_hash with similarity score.
type Suggestion struct {
	MatchHash string
	Score     float64
}

// RankSuggestions filters by schema-driven bilateral checks, then ranks by cosine similarity.
// Groups into tiers, shuffles within tiers, returns top N.
func RankSuggestions(schema *gh.PoolSchema, me Record, records []Record, limit int) []Suggestion {
	var scored []Suggestion
	for _, r := range records {
		if r.MatchHash == me.MatchHash {
			continue
		}
		// Schema-driven filter
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

	// Tier-based shuffling: 10 tiers, shuffle within each
	if len(scored) > 10 {
		tierSize := len(scored) / 10
		for t := 0; t < 10; t++ {
			start := t * tierSize
			end := start + tierSize
			if t == 9 {
				end = len(scored)
			}
			tier := scored[start:end]
			rand.Shuffle(len(tier), func(i, j int) {
				tier[i], tier[j] = tier[j], tier[i]
			})
		}
	}

	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
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
