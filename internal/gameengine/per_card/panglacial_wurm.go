package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPanglacialWurm registers a stub handler for Panglacial Wurm.
//
// Oracle text:
//
//	Trample
//	While you're searching your library, you may cast Panglacial Wurm
//	from your library.
//
// This is an extremely niche mechanic — casting from library mid-tutor
// requires opening a priority window inside the tutor resolution, which
// the engine does not currently support (resolveTutor does not yield).
//
// This stub logs the existence of the card so tests can verify the
// handler is registered. Full implementation deferred.
func registerPanglacialWurm(r *Registry) {
	r.OnCast("Panglacial Wurm", panglacialWurmCast)
}

func panglacialWurmCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "panglacial_wurm"
	if gs == nil || item == nil {
		return
	}
	emit(gs, slug, "Panglacial Wurm", map[string]interface{}{
		"note": "cast_from_library_mid_tutor_not_yet_implemented",
	})
	emitPartial(gs, slug, "Panglacial Wurm",
		"mid_tutor_cast_window_not_implemented")
}
