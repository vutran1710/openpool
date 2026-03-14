package crypto

import (
	"crypto/sha256"
	"encoding/hex"
)

type MerkleTree struct {
	Leaves [][]byte
	Layers [][][]byte
	Root   []byte
}

func NewMerkleTree(leaves []string) *MerkleTree {
	hashedLeaves := make([][]byte, len(leaves))
	for i, leaf := range leaves {
		h := sha256.Sum256([]byte(leaf))
		hashedLeaves[i] = h[:]
	}

	tree := &MerkleTree{Leaves: hashedLeaves}
	tree.build()
	return tree
}

func (t *MerkleTree) build() {
	current := make([][]byte, len(t.Leaves))
	copy(current, t.Leaves)
	t.Layers = append(t.Layers, current)

	for len(current) > 1 {
		var next [][]byte
		for i := 0; i < len(current); i += 2 {
			if i+1 < len(current) {
				next = append(next, hashPair(current[i], current[i+1]))
			} else {
				next = append(next, hashPair(current[i], current[i]))
			}
		}
		t.Layers = append(t.Layers, next)
		current = next
	}

	t.Root = current[0]
}

func (t *MerkleTree) RootHex() string {
	return hex.EncodeToString(t.Root)
}

type MerkleProof struct {
	LeafHash  string   `json:"leaf_hash"`
	LeafIndex int      `json:"leaf_index"`
	Siblings  []string `json:"siblings"`
	Root      string   `json:"root"`
}

func (t *MerkleTree) GenerateProof(leafIndex int) *MerkleProof {
	if leafIndex < 0 || leafIndex >= len(t.Leaves) {
		return nil
	}

	proof := &MerkleProof{
		LeafHash:  hex.EncodeToString(t.Leaves[leafIndex]),
		LeafIndex: leafIndex,
		Root:      t.RootHex(),
	}

	idx := leafIndex
	for _, layer := range t.Layers[:len(t.Layers)-1] {
		var siblingIdx int
		if idx%2 == 0 {
			siblingIdx = idx + 1
		} else {
			siblingIdx = idx - 1
		}

		if siblingIdx >= len(layer) {
			siblingIdx = idx
		}

		proof.Siblings = append(proof.Siblings, hex.EncodeToString(layer[siblingIdx]))
		idx /= 2
	}

	return proof
}

func VerifyMerkleProof(proof *MerkleProof) bool {
	current, err := hex.DecodeString(proof.LeafHash)
	if err != nil {
		return false
	}

	idx := proof.LeafIndex
	for _, sibHex := range proof.Siblings {
		sibling, err := hex.DecodeString(sibHex)
		if err != nil {
			return false
		}

		if idx%2 == 0 {
			current = hashPair(current, sibling)
		} else {
			current = hashPair(sibling, current)
		}
		idx /= 2
	}

	return hex.EncodeToString(current) == proof.Root
}

func BuildIdentityTree(pubKeyHex, provider, providerUserID, gender string) *MerkleTree {
	return NewMerkleTree([]string{pubKeyHex, provider, providerUserID, gender})
}

func hashPair(a, b []byte) []byte {
	combined := make([]byte, len(a)+len(b))
	copy(combined, a)
	copy(combined[len(a):], b)
	h := sha256.Sum256(combined)
	return h[:]
}
