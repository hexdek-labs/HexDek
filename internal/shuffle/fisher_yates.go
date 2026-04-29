// Package shuffle provides Fisher-Yates shuffling backed by crypto/rand.
//
// The Fisher-Yates algorithm guarantees a uniform distribution over all
// permutations of the input slice, given a uniform random source. We use
// crypto/rand as the entropy source because it's cryptographically secure
// and resistant to seed prediction.
//
// For multi-party games requiring trustless shuffling, a commit-reveal
// scheme on top of this primitive should be added at the game layer; this
// package is just the shuffle.
package shuffle

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// Shuffle randomizes the order of cards in-place using Fisher-Yates with
// crypto/rand entropy. It returns an error only if the entropy source fails.
func Shuffle[T any](cards []T) error {
	for i := len(cards) - 1; i > 0; i-- {
		j, err := randIntN(i + 1)
		if err != nil {
			return fmt.Errorf("shuffle entropy: %w", err)
		}
		cards[i], cards[j] = cards[j], cards[i]
	}
	return nil
}

// randIntN returns a uniform random int in [0, n).
//
// We avoid math/rand entirely. The standard pattern of taking
// crypto/rand.Read into a uint64 and modding by n introduces a small
// modulo bias when n is not a power of two. For 100-card libraries the
// bias is negligible (< 1e-17), but we use rejection sampling anyway for
// correctness and consistency.
func randIntN(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("randIntN: n must be > 0, got %d", n)
	}
	if n == 1 {
		return 0, nil
	}

	// Largest multiple of n that fits in uint64.
	max := uint64(n)
	rangeMax := (^uint64(0)) - ((^uint64(0)) % max)

	var buf [8]byte
	for {
		if _, err := rand.Read(buf[:]); err != nil {
			return 0, fmt.Errorf("crypto/rand.Read: %w", err)
		}
		v := binary.BigEndian.Uint64(buf[:])
		if v < rangeMax {
			return int(v % max), nil
		}
		// Reject and re-roll on bias-region samples.
	}
}
