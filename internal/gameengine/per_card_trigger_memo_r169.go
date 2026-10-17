package gameengine

// r169 — accessors for the per_card trigger-handler memo slot.
//
// internal/gameengine/per_card dispatches triggered abilities by walking
// every permanent on every battlefield for every event and asking its
// registry whether that card's name has a handler. The overwhelming
// majority of permanents have none, and the question's answer is
// constant for the life of a (card name, registry) pair — but the
// lookup is not free: an RWMutex read lock, a sync.Map load keyed on the
// display name, and one or more string-map probes, per permanent, per
// event.
//
// per_card caches its answer here. gameengine deliberately knows nothing
// about the shape of the value: per_card.TriggerHandler lives in a
// package that imports this one, so a typed field would be an import
// cycle. This is the "any slot" half of that arrangement; the keying,
// the invalidation and the whole correctness argument live in
// per_card/trigger_memo_r169.go.

// PerCardTriggerMemo returns the opaque memo value previously stored by
// SetPerCardTriggerMemo, or nil.
func (c *Card) PerCardTriggerMemo() any {
	if c == nil {
		return nil
	}
	return c.perCardTriggerMemo
}

// SetPerCardTriggerMemo stores an opaque memo value on this card.
func (c *Card) SetPerCardTriggerMemo(v any) {
	if c == nil {
		return
	}
	c.perCardTriggerMemo = v
}
