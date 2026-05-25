package moxfield

import (
	"encoding/json"
	"strings"
	"testing"
)

// fixtures_r60_test.go — pins formatDecklist behavior against realistic
// Moxfield v3 API JSON shapes. The Moxfield response is fat (per-card
// `prices`, `isFoil`, `isEtched`, `useCmcOverride`, alt-printing fields,
// the full `card_faces` array, set/collector info, etc.); the importer
// only cares about commanders/mainboard/sideboard/companions and the
// card's display Name. These tests pin that:
//
//   - DFC names round-trip with the "//" separator (so downstream
//     Scryfall lookup still works for `Malakir Rebirth // Malakir Mire`).
//   - Partner decks emit exactly two COMMANDER: lines in deterministic
//     alphabetical order (the downstream deckparser treats the first as
//     primary — a map-iteration flip would silently re-assign primary).
//   - Companion goes through as `// Companion:` and is NOT promoted to
//     commander or mainboard.
//   - Foil / etched / price / scryfall_id / alt-printing fields are
//     parsed without erroring AND don't leak into the output text.
//   - Real-shape (mixed: partner + DFCs + companion + sideboard + prices)
//     produces a single coherent decklist whose section ordering and
//     entry ordering are deterministic.

func parseFixture(t *testing.T, raw string) *apiResponse {
	t.Helper()
	var data apiResponse
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return &data
}

// ---------------------------------------------------------------------------
// DFC commander + DFC mainboard entries — the "//" separator must survive
// ---------------------------------------------------------------------------

const fixtureDFC = `{
	"name": "Valki DFC Test",
	"format": "commander",
	"boards": {
		"commanders": {
			"count": 1,
			"cards": {
				"c1": {
					"quantity": 1,
					"isFoil": false,
					"isEtched": false,
					"card": {
						"name": "Valki, God of Lies // Tibalt, Cosmic Impostor",
						"scryfall_id": "59cf0fc7-ca5b-4189-b074-77a2d3e5c2b0",
						"set": "khm",
						"prices": {"usd": "8.99", "usd_foil": "21.00", "tix": "1.20"},
						"card_faces": [
							{"name": "Valki, God of Lies", "mana_cost": "{1}{B}"},
							{"name": "Tibalt, Cosmic Impostor", "mana_cost": "{4}{B}{R}"}
						]
					}
				}
			}
		},
		"mainboard": {
			"count": 2,
			"cards": {
				"m1": {
					"quantity": 1,
					"card": {
						"name": "Malakir Rebirth // Malakir Mire",
						"prices": {"usd": "0.45"}
					}
				},
				"m2": {
					"quantity": 4,
					"card": {
						"name": "Bonecrusher Giant // Stomp",
						"isFoil": true
					}
				}
			}
		}
	}
}`

func TestFixture_DFCNamesRoundTripWithSlashSeparator(t *testing.T) {
	data := parseFixture(t, fixtureDFC)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, want := range []string{
		"COMMANDER: Valki, God of Lies // Tibalt, Cosmic Impostor",
		"1 Malakir Rebirth // Malakir Mire",
		"4 Bonecrusher Giant // Stomp",
	} {
		if !containsLine(out, want) {
			t.Errorf("missing DFC line %q in output:\n%s", want, out)
		}
	}
	// The single-face component names must NOT appear by themselves —
	// emitting "1 Valki, God of Lies" without the back face would route
	// Scryfall lookup to the wrong card on a DFC commander.
	for _, forbidden := range []string{
		"COMMANDER: Valki, God of Lies\n",
		"COMMANDER: Tibalt, Cosmic Impostor\n",
		"1 Malakir Rebirth\n",
		"1 Malakir Mire\n",
	} {
		if strings.Contains(out, forbidden) {
			t.Errorf("DFC back/front face leaked into output as separate line %q in:\n%s",
				forbidden, out)
		}
	}
}

func TestFixture_DFC_PriceAndFoilFieldsDoNotLeak(t *testing.T) {
	data := parseFixture(t, fixtureDFC)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, leaked := range []string{
		"usd", "USD", "$", "8.99", "21.00", "tix",
		"isFoil", "true", "false", "scryfall_id", "set",
		"card_faces", "mana_cost",
	} {
		if strings.Contains(out, leaked) {
			t.Errorf("Moxfield-noise field %q leaked into deck text:\n%s", leaked, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Partner — two commanders, alphabetical-deterministic order
// ---------------------------------------------------------------------------

const fixturePartner = `{
	"name": "Akiri + Bruse Partner",
	"format": "commander",
	"boards": {
		"commanders": {
			"count": 2,
			"cards": {
				"bruse": {
					"quantity": 1,
					"card": {
						"name": "Bruse Tarl, Boorish Herder",
						"prices": {"usd": "2.50"}
					}
				},
				"akiri": {
					"quantity": 1,
					"card": {
						"name": "Akiri, Line-Slinger",
						"prices": {"usd": "3.10"}
					}
				}
			}
		},
		"mainboard": {
			"count": 1,
			"cards": {
				"m1": {"quantity": 1, "card": {"name": "Sol Ring"}}
			}
		}
	}
}`

func TestFixture_PartnerEmitsExactlyTwoCommanderLines(t *testing.T) {
	data := parseFixture(t, fixturePartner)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	count := strings.Count(out, "COMMANDER: ")
	if count != 2 {
		t.Fatalf("partner deck must emit exactly 2 COMMANDER lines, got %d:\n%s", count, out)
	}
}

func TestFixture_PartnerCommanderOrderIsDeterministicAlphabetical(t *testing.T) {
	// formatDecklist iterates the entries map in sorted KEY order. The
	// fixture uses keys "akiri" / "bruse" (lowercase) so Akiri comes
	// first. Run formatDecklist multiple times; the order MUST be stable.
	// (Without the sort, Go's randomized map iteration would flip the
	// primary commander across fetches.)
	data := parseFixture(t, fixturePartner)
	first, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	idxAkiri := strings.Index(first, "COMMANDER: Akiri, Line-Slinger")
	idxBruse := strings.Index(first, "COMMANDER: Bruse Tarl, Boorish Herder")
	if idxAkiri < 0 || idxBruse < 0 {
		t.Fatalf("missing partner commanders:\n%s", first)
	}
	if idxAkiri >= idxBruse {
		t.Errorf("partner ordering: Akiri (entry key %q) should sort before Bruse (entry key %q); got Akiri@%d Bruse@%d",
			"akiri", "bruse", idxAkiri, idxBruse)
	}
	for i := 0; i < 20; i++ {
		out, err := formatDecklist(data)
		if err != nil {
			t.Fatalf("formatDecklist iter %d: %v", i, err)
		}
		if out != first {
			t.Fatalf("formatDecklist is not deterministic across calls; iter %d differs from first", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Companion — Lurrus stays a companion, never promoted to commander
// ---------------------------------------------------------------------------

const fixtureCompanion = `{
	"name": "Lurrus Companion Deck",
	"format": "commander",
	"boards": {
		"commanders": {
			"count": 1,
			"cards": {
				"c1": {"quantity": 1, "card": {"name": "Edgar Markov"}}
			}
		},
		"mainboard": {
			"count": 1,
			"cards": {
				"m1": {"quantity": 1, "card": {"name": "Sol Ring"}}
			}
		},
		"companions": {
			"count": 1,
			"cards": {
				"comp1": {
					"quantity": 1,
					"card": {
						"name": "Lurrus of the Dream-Den",
						"prices": {"usd": "3.40"}
					}
				}
			}
		}
	}
}`

func TestFixture_CompanionEmitsCompanionMarker(t *testing.T) {
	data := parseFixture(t, fixtureCompanion)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !containsLine(out, "// Companion: 1 Lurrus of the Dream-Den") {
		t.Errorf("expected `// Companion:` marker for Lurrus; got:\n%s", out)
	}
}

func TestFixture_CompanionNotPromotedToCommanderOrMainboard(t *testing.T) {
	data := parseFixture(t, fixtureCompanion)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if strings.Contains(out, "COMMANDER: Lurrus") {
		t.Errorf("Lurrus must NOT be promoted to COMMANDER:\n%s", out)
	}
	if containsLine(out, "1 Lurrus of the Dream-Den") {
		t.Errorf("Lurrus must NOT appear as a bare mainboard entry — only as `// Companion:`:\n%s", out)
	}
	// Edgar must still be primary commander.
	if !containsLine(out, "COMMANDER: Edgar Markov") {
		t.Errorf("primary commander Edgar Markov missing:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Sideboard — emitted with `// Sideboard:` marker
// ---------------------------------------------------------------------------

const fixtureSideboard = `{
	"name": "With Maybeboard",
	"format": "commander",
	"boards": {
		"commanders": {
			"count": 1,
			"cards": {"c1": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}
		},
		"mainboard": {
			"count": 1,
			"cards": {"m1": {"quantity": 1, "card": {"name": "Sol Ring"}}}
		},
		"sideboard": {
			"count": 2,
			"cards": {
				"s1": {"quantity": 1, "card": {"name": "Tezzeret the Seeker"}},
				"s2": {"quantity": 2, "card": {"name": "Wrath of God"}}
			}
		}
	}
}`

func TestFixture_SideboardMarkerAndQuantities(t *testing.T) {
	data := parseFixture(t, fixtureSideboard)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, want := range []string{
		"// Sideboard: 1 Tezzeret the Seeker",
		"// Sideboard: 2 Wrath of God",
	} {
		if !containsLine(out, want) {
			t.Errorf("missing sideboard line %q in:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Mixed real-shape: partner + DFC mainboard + companion + sideboard + noise
// ---------------------------------------------------------------------------

const fixtureMixed = `{
	"name": "Partners with DFC + Companion",
	"format": "commander",
	"id": "abc123xyz",
	"publicId": "abc123xyz",
	"createdAtUtc": "2024-01-01T00:00:00Z",
	"viewCount": 0,
	"likeCount": 0,
	"areCommentsEnabled": true,
	"boards": {
		"commanders": {
			"count": 2,
			"cards": {
				"a": {"quantity": 1, "card": {"name": "Akiri, Line-Slinger", "prices": {"usd": "3.10"}}},
				"b": {"quantity": 1, "card": {"name": "Silas Renn, Seeker Adept", "prices": {"usd": "2.40"}}}
			}
		},
		"mainboard": {
			"count": 3,
			"cards": {
				"x": {"quantity": 1, "card": {"name": "Brazen Borrower // Petty Theft", "isFoil": true, "prices": {"usd": "4.00", "usd_foil": "12.00"}}},
				"y": {"quantity": 1, "card": {"name": "Sol Ring", "isFoil": false}},
				"z": {"quantity": 4, "card": {"name": "Island"}}
			}
		},
		"companions": {
			"count": 1,
			"cards": {
				"c": {"quantity": 1, "card": {"name": "Jegantha, the Wellspring"}}
			}
		},
		"sideboard": {
			"count": 1,
			"cards": {
				"s": {"quantity": 1, "card": {"name": "Pithing Needle"}}
			}
		}
	}
}`

func TestFixture_MixedRealShape_AllSectionsPresent(t *testing.T) {
	data := parseFixture(t, fixtureMixed)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, want := range []string{
		"COMMANDER: Akiri, Line-Slinger",
		"COMMANDER: Silas Renn, Seeker Adept",
		"1 Brazen Borrower // Petty Theft",
		"1 Sol Ring",
		"4 Island",
		"// Sideboard: 1 Pithing Needle",
		"// Companion: 1 Jegantha, the Wellspring",
	} {
		if !containsLine(out, want) {
			t.Errorf("mixed-shape: missing line %q in:\n%s", want, out)
		}
	}
}

func TestFixture_MixedRealShape_SectionOrderIsCommandersMainSideCompanion(t *testing.T) {
	// formatDecklist's section ordering is: commanders → mainboard →
	// sideboard → companions. Pin it so a refactor that swaps the
	// writeEntries call order is caught.
	data := parseFixture(t, fixtureMixed)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	idxCommander := strings.Index(out, "COMMANDER: Akiri")
	idxMain := strings.Index(out, "4 Island")
	idxSide := strings.Index(out, "// Sideboard:")
	idxComp := strings.Index(out, "// Companion:")
	if idxCommander < 0 || idxMain < 0 || idxSide < 0 || idxComp < 0 {
		t.Fatalf("missing one of the section markers:\n%s", out)
	}
	if !(idxCommander < idxMain && idxMain < idxSide && idxSide < idxComp) {
		t.Errorf("section order broken: COMMANDER@%d main@%d sideboard@%d companion@%d (want strictly ascending)",
			idxCommander, idxMain, idxSide, idxComp)
	}
}

func TestFixture_MixedRealShape_NoNoiseFieldsLeak(t *testing.T) {
	// publicId / createdAtUtc / viewCount / likeCount / areCommentsEnabled
	// — none of the deck-metadata fields the API returns should ever
	// appear in the .txt output.
	data := parseFixture(t, fixtureMixed)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, leaked := range []string{
		"publicId", "createdAtUtc", "viewCount", "likeCount",
		"areCommentsEnabled", "abc123xyz", "2024-01-01",
		"prices", "isFoil", "usd_foil", "12.00", "4.00",
	} {
		if strings.Contains(out, leaked) {
			t.Errorf("API noise field %q leaked into deck text:\n%s", leaked, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Legacy top-level boards (pre-v3 wrapper) still work — at least one
// production cache file uses this shape, and the apiResponse fallback
// path is non-obvious; pin it so a refactor doesn't silently break it.
// ---------------------------------------------------------------------------

const fixtureLegacyTopLevel = `{
	"name": "Pre-v3 Layout",
	"format": "commander",
	"commanders": {
		"c1": {"quantity": 1, "card": {"name": "Yuriko, the Tiger's Shadow"}}
	},
	"mainboard": {
		"m1": {"quantity": 99, "card": {"name": "Island"}}
	}
}`

func TestFixture_LegacyTopLevelBoardsStillWork(t *testing.T) {
	data := parseFixture(t, fixtureLegacyTopLevel)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !containsLine(out, "COMMANDER: Yuriko, the Tiger's Shadow") {
		t.Errorf("legacy top-level commander missing:\n%s", out)
	}
	if !containsLine(out, "99 Island") {
		t.Errorf("legacy top-level mainboard entry missing:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// v3 takes precedence when both are present
// ---------------------------------------------------------------------------

const fixtureBothLayouts = `{
	"name": "Both Shapes",
	"format": "commander",
	"commanders": {
		"old": {"quantity": 1, "card": {"name": "Old Commander"}}
	},
	"boards": {
		"commanders": {
			"count": 1,
			"cards": {
				"new": {"quantity": 1, "card": {"name": "New Commander"}}
			}
		},
		"mainboard": {
			"count": 1,
			"cards": {
				"m1": {"quantity": 1, "card": {"name": "Sol Ring"}}
			}
		}
	}
}`

func TestFixture_V3BoardsTakePrecedenceOverLegacyTopLevel(t *testing.T) {
	data := parseFixture(t, fixtureBothLayouts)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !containsLine(out, "COMMANDER: New Commander") {
		t.Errorf("v3 commander missing:\n%s", out)
	}
	if containsLine(out, "COMMANDER: Old Commander") {
		t.Errorf("legacy commander should be shadowed by v3 boards:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Quantity / name edge cases
// ---------------------------------------------------------------------------

const fixtureQuantityZero = `{
	"name": "Quantity Zero",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"c1": {"quantity": 1, "card": {"name": "Krenko, Mob Boss"}}}},
		"mainboard": {
			"count": 1,
			"cards": {
				"m1": {"quantity": 0, "card": {"name": "Should Be Skipped"}},
				"m2": {"quantity": 1, "card": {"name": "Sol Ring"}}
			}
		}
	}
}`

func TestFixture_QuantityZeroEntriesAreSkipped(t *testing.T) {
	data := parseFixture(t, fixtureQuantityZero)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if strings.Contains(out, "Should Be Skipped") {
		t.Errorf("quantity=0 entry leaked into output:\n%s", out)
	}
	if !containsLine(out, "1 Sol Ring") {
		t.Errorf("non-zero entry missing:\n%s", out)
	}
}

const fixtureEmptyName = `{
	"name": "Empty Card Names",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"c1": {"quantity": 1, "card": {"name": "Karador, Ghost Chieftain"}}}},
		"mainboard": {
			"count": 1,
			"cards": {
				"good": {"quantity": 1, "card": {"name": "Sol Ring"}},
				"orphan": {"quantity": 1, "card": {"name": ""}}
			}
		}
	}
}`

func TestFixture_EmptyCardNamesAreSkipped(t *testing.T) {
	data := parseFixture(t, fixtureEmptyName)
	out, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	// One empty-name line at qty 1 would emit "1 " — guard against that.
	for _, line := range strings.Split(out, "\n") {
		if line == "1 " || line == "1" {
			t.Errorf("orphan blank-name entry leaked into output:\n%s", out)
		}
	}
	if !containsLine(out, "1 Sol Ring") {
		t.Errorf("non-blank entry missing:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// A totally empty deck must error rather than silently writing nothing
// (the importer would otherwise produce a 2-line "# header" file).
// ---------------------------------------------------------------------------

func TestFixture_EmptyDeckErrors(t *testing.T) {
	data := parseFixture(t, `{"name":"Empty","format":"commander","boards":{}}`)
	if _, err := formatDecklist(data); err == nil {
		t.Fatal("expected error for completely empty deck")
	}
}
