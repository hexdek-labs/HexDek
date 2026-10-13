package gameengine

import "strings"

// ExpireKeywordGrantsEOT removes "until end of turn" keyword grants that
// were recorded on Permanent.Flags.
//
// CR §514.2 — "until end of turn" effects end during the cleanup step.
// The sweep in phases.go clears Permanent.GrantedAbilities and the
// until-EOT entries in Permanent.Modifications, but it has never touched
// the Flags map, which is the other surface HasKeyword reads
// (Flags["kw:<name>"], combat.go:80). A handler that granted a temporary
// keyword through that surface therefore granted it for the rest of the
// game.
//
// A grant opts into expiry by also setting Flags["kw_eot:<name>"].
// Permanent keyword flags — a token minted with flying, a face-down
// permanent's disguise ward — set no such marker and are left alone, so
// this cannot strip a keyword that was never temporary.
func ExpireKeywordGrantsEOT(gs *GameState) {
	if gs == nil {
		return
	}
	const marker = "kw_eot:"
	cleared := false
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || len(p.Flags) == 0 {
				continue
			}
			var expired []string
			for k := range p.Flags {
				if strings.HasPrefix(k, marker) {
					expired = append(expired, strings.TrimPrefix(k, marker))
				}
			}
			for _, kw := range expired {
				delete(p.Flags, marker+kw)
				delete(p.Flags, "kw:"+kw)
				cleared = true
			}
			// Drop the matching GrantedAbilities entries too, so the
			// grant is fully expired by this call alone rather than
			// relying on the separate wholesale clear in the cleanup
			// sweep. HasKeyword reads both surfaces; leaving one behind
			// would expire nothing.
			if len(expired) > 0 && len(p.GrantedAbilities) > 0 {
				drop := make(map[string]bool, len(expired))
				for _, kw := range expired {
					drop[kw] = true
					drop[strings.ReplaceAll(kw, "_", " ")] = true
				}
				kept := p.GrantedAbilities[:0]
				for _, g := range p.GrantedAbilities {
					if drop[g] {
						continue
					}
					kept = append(kept, g)
				}
				p.GrantedAbilities = kept
			}
		}
	}
	if cleared {
		gs.InvalidateCharacteristicsCache()
	}
}
