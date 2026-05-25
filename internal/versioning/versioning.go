// Package versioning implements a git-for-decks versioning DAG with
// Bayesian prior inheritance. Each deck version is a node in a DAG;
// updating a deck creates a child node that inherits its parent's
// TrueSkill μ with σ inflated proportional to the card delta.
//
// The DAG tracks lineage, card hashes, and HEAD pointers for the
// leaderboard. Rating data is stored alongside version nodes so the
// showmatch/tournament system can look up the prior for any deck.
package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/trueskill"
)

// VersionNode is a single node in the deck versioning DAG.
type VersionNode struct {
	// Hash is a content-addressable ID derived from the card list.
	Hash string `json:"hash"`

	// ParentHash is the hash of the parent version. Empty for root.
	ParentHash string `json:"parent_hash,omitempty"`

	// Owner is the deck owner (directory name).
	Owner string `json:"owner"`

	// DeckID is the deck filename stem.
	DeckID string `json:"deck_id"`

	// Commander is the commander card name.
	Commander string `json:"commander"`

	// Version is the incrementing version number (1, 2, 3...).
	Version int `json:"version"`

	// CardCount is the number of cards in this version.
	CardCount int `json:"card_count"`

	// CardDelta is the number of cards changed from the parent.
	CardDelta int `json:"card_delta"`

	// Rating is the TrueSkill prior for this version.
	Rating trueskill.Rating `json:"rating"`

	// GamesPlayed is how many rated games this version has played.
	GamesPlayed int `json:"games_played"`

	// CreatedAt is when this version was recorded.
	CreatedAt string `json:"created_at"`

	// IsHead is true if this is the current (latest) version.
	IsHead bool `json:"is_head"`

	// Archetype is the Freya primary_archetype label at the moment
	// strategy.json was last produced for this version (e.g.
	// "Combo / Infinite", "Lands Matter"). Empty when Freya hasn't
	// classified this version yet — older nodes that predate this
	// field stay empty until the next runFreya completes and calls
	// UpdateArchetype. Populated by hexapi.runFreya's success path,
	// consumed by the /archetype-history timeline endpoint.
	Archetype string `json:"archetype,omitempty"`
}

// DeckDAG holds the full version history for all decks.
type DeckDAG struct {
	// Nodes indexed by hash.
	Nodes map[string]*VersionNode `json:"nodes"`

	// Heads maps owner/deckID to the current HEAD hash.
	Heads map[string]string `json:"heads"`
}

// NewDeckDAG creates an empty DAG.
func NewDeckDAG() *DeckDAG {
	return &DeckDAG{
		Nodes: make(map[string]*VersionNode),
		Heads: make(map[string]string),
	}
}

// headKey returns the canonical key for a deck's HEAD pointer.
func headKey(owner, deckID string) string {
	return owner + "/" + deckID
}

// HashCardList computes a deterministic hash of a card list.
func HashCardList(cards []string) string {
	sorted := make([]string, len(cards))
	copy(sorted, cards)
	sort.Strings(sorted)
	h := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(h[:16])
}

// CardDelta computes how many cards differ between two sorted card lists.
func CardDelta(oldCards, newCards []string) int {
	old := make(map[string]int, len(oldCards))
	for _, c := range oldCards {
		old[c]++
	}
	new_ := make(map[string]int, len(newCards))
	for _, c := range newCards {
		new_[c]++
	}

	delta := 0
	all := make(map[string]bool)
	for k := range old {
		all[k] = true
	}
	for k := range new_ {
		all[k] = true
	}
	for k := range all {
		diff := new_[k] - old[k]
		if diff < 0 {
			diff = -diff
		}
		delta += diff
	}
	return delta
}

// RegisterVersion adds a new version node to the DAG. If the card list
// hasn't changed (same hash), returns the existing node. Otherwise,
// creates a child node with Bayesian prior inheritance from the parent.
func (dag *DeckDAG) RegisterVersion(owner, deckID, commander string, cards []string) *VersionNode {
	hash := HashCardList(cards)

	// Already exists — return as-is.
	if existing, ok := dag.Nodes[hash]; ok {
		return existing
	}

	key := headKey(owner, deckID)
	parentHash := dag.Heads[key]

	node := &VersionNode{
		Hash:      hash,
		Owner:     owner,
		DeckID:    deckID,
		Commander: commander,
		CardCount: len(cards),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		IsHead:    true,
	}

	if parentHash != "" {
		parent, ok := dag.Nodes[parentHash]
		if ok {
			parent.IsHead = false
			node.ParentHash = parentHash
			node.Version = parent.Version + 1

			// Compute card delta for sigma inflation.
			oldCards := dag.reconstructCardList(parentHash)
			if oldCards != nil {
				node.CardDelta = CardDelta(oldCards, cards)
			} else {
				node.CardDelta = len(cards) / 5
			}

			// Bayesian prior inheritance: μ carries, σ inflates by delta.
			node.Rating = trueskill.InheritRating(parent.Rating, node.CardDelta)
		} else {
			node.Version = 1
			node.Rating = trueskill.DefaultRating()
		}
	} else {
		node.Version = 1
		node.Rating = trueskill.DefaultRating()
	}

	dag.Nodes[hash] = node
	dag.Heads[key] = hash

	return node
}

// GetHead returns the current HEAD version for a deck, or nil.
func (dag *DeckDAG) GetHead(owner, deckID string) *VersionNode {
	hash, ok := dag.Heads[headKey(owner, deckID)]
	if !ok {
		return nil
	}
	return dag.Nodes[hash]
}

// GetLineage returns the version history for a deck from HEAD back to
// root, ordered newest-first.
func (dag *DeckDAG) GetLineage(owner, deckID string) []*VersionNode {
	hash, ok := dag.Heads[headKey(owner, deckID)]
	if !ok {
		return nil
	}

	var lineage []*VersionNode
	for hash != "" {
		node, ok := dag.Nodes[hash]
		if !ok {
			break
		}
		lineage = append(lineage, node)
		hash = node.ParentHash
	}
	return lineage
}

// Leaderboard returns all HEAD versions sorted by conservative rating.
func (dag *DeckDAG) Leaderboard() []*VersionNode {
	var heads []*VersionNode
	for _, hash := range dag.Heads {
		if node, ok := dag.Nodes[hash]; ok {
			heads = append(heads, node)
		}
	}
	sort.Slice(heads, func(i, j int) bool {
		return heads[i].Rating.Conservative() > heads[j].Rating.Conservative()
	})
	return heads
}

// UpdateRating updates the rating for a version node after a game.
func (dag *DeckDAG) UpdateRating(hash string, rating trueskill.Rating, gamesPlayed int) {
	if node, ok := dag.Nodes[hash]; ok {
		node.Rating = rating
		node.GamesPlayed = gamesPlayed
	}
}

// UpdateArchetype stamps Freya's primary_archetype label onto the
// version node identified by hash. Called by hexapi.runFreya's
// success path once strategy.json has been written, so the DAG
// always tracks "what archetype was Freya calling this version at
// creation time." Consecutive versions with different Archetype
// values surface as reclassification events in the
// /api/decks/{owner}/{id}/archetype-history timeline.
//
// No-op when the hash is unknown (the runFreya callback can't
// guarantee the DAG still contains the version it ran against —
// concurrent registerDeckVersion calls could have rotated HEAD).
func (dag *DeckDAG) UpdateArchetype(hash, archetype string) {
	if node, ok := dag.Nodes[hash]; ok {
		node.Archetype = archetype
	}
}

// LookupByCommander returns the HEAD version for a given commander name.
// Searches all heads for a matching commander. Returns nil if not found.
func (dag *DeckDAG) LookupByCommander(commander string) *VersionNode {
	lc := strings.ToLower(commander)
	for _, hash := range dag.Heads {
		node := dag.Nodes[hash]
		if node != nil && strings.ToLower(node.Commander) == lc {
			return node
		}
	}
	return nil
}

func (dag *DeckDAG) reconstructCardList(hash string) []string {
	// Card lists are not stored in the DAG (too large). This is used
	// for delta computation during RegisterVersion. The caller computes
	// the delta from the file system. Return nil to signal "unknown".
	return nil
}

// --- Persistence ---

// LoadDAG reads the DAG from a JSON file.
func LoadDAG(dir string) (*DeckDAG, error) {
	path := filepath.Join(dir, "deck_dag.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewDeckDAG(), nil
		}
		return nil, fmt.Errorf("read DAG: %w", err)
	}
	dag := NewDeckDAG()
	if err := json.Unmarshal(data, dag); err != nil {
		return nil, fmt.Errorf("parse DAG: %w", err)
	}
	return dag, nil
}

// SaveDAG writes the DAG to a JSON file atomically.
func SaveDAG(dir string, dag *DeckDAG) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "deck_dag.json")
	data, err := json.MarshalIndent(dag, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// InheritInfo returns a human-readable description of the prior
// inheritance for a version node.
func InheritInfo(node *VersionNode) string {
	if node.ParentHash == "" {
		return fmt.Sprintf("v%d (root): μ=%.1f σ=%.1f conservative=%.1f",
			node.Version, node.Rating.Mu, node.Rating.Sigma, node.Rating.Conservative())
	}
	return fmt.Sprintf("v%d (parent=%s, Δ=%d cards): μ=%.1f σ=%.1f conservative=%.1f",
		node.Version, node.ParentHash[:8], node.CardDelta,
		node.Rating.Mu, node.Rating.Sigma, node.Rating.Conservative())
}

// SigmaInflation computes the sigma inflation for a given card delta.
// Exported for testing.
func SigmaInflation(cardDelta int) float64 {
	return math.Min(float64(cardDelta)*0.5, 25.0/6.0)
}
