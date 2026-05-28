package per_card

// Batch AI (R60) — 4 §400.7c-affected exile-then-cast cards wired
// through the post-#685 owner-routing discipline (plus a Possibility
// Storm bug fix in chaos_cascade.go itself, which is already
// auto-registered via the existing batch registration). See
// per_card_batch_ai_r60.go for per-card oracle text and design notes.
//
// init() registers against the global registry and adds a Reset hook
// so handlers survive per_card.Reset() in tests — mirrors
// zz_batch_y_r60_register.go's pattern.

func init() {
	RegisterBatchAIR60(Global())
	AddResetHook(RegisterBatchAIR60)
}

// RegisterBatchAIR60 registers the batch-AI R60 handlers.
func RegisterBatchAIR60(r *Registry) {
	if r == nil {
		return
	}
	registerBribery(r)
	registerHostageTaker(r)
	registerKnowledgePool(r)
	registerMindsDesire(r)
}
