# Layer Harness Report

Pool: **31,639 real cards** (parser at 100% GREEN).

Every card has been run through the parser, and every ability has been
tagged with the §613 layer it operates in (where applicable). Layers
1-5 govern copy/control/text/type/color. Layer 6 is ability add/remove.
Layers 7a-7e are the P/T sublayer cascade.

## Layer distribution (cards that touch each layer)

| Layer | Cards | % | Description |
|---|---:|---:|---|
| **1** | 6 | 0.02% | copy effects |
| **2** | 76 | 0.24% | control-changing |
| **3** | 0 | 0.00% | text-changing |
| **4** | 166 | 0.52% | type/subtype/supertype changes |
| **5** | 1 | 0.00% | color-changing |
| **6** | 8,751 | 27.66% | ability add/remove |
| **7a** | 188 | 0.59% | characteristic-defining P/T |
| **7b** | 47 | 0.15% | P/T set (becomes N/N) |
| **7c** | 2,100 | 6.64% | P/T modify (anthems, +N/+N) |
| **7d** | 1,511 | 4.78% | counters (+1/+1, -1/-1) |
| **7e** | 0 | 0.00% | P/T switching |

## Multi-layer cards

Cards that touch 2+ layers simultaneously: **1,354** (4.28%).

Cards with at least one layered effect: **11,475** (36.27%).

Cards with zero layered effects (vanilla creatures, instants/sorceries, spell-only cards): **20,164** (63.73%).

## Layer signature for top-50 cEDH-ranked cards

Shows which layers each of the most-played cards operates in.

| # | Card | Layers touched |
|---:|---|---|
| 1 | Sol Ring | — |
| 2 | Command Tower | — |
| 3 | Arcane Signet | — |
| 9 | Exotic Orchard | — |
| 10 | Reliquary Tower | — |
| 11 | Swords to Plowshares | — |
| 12 | Swiftfoot Boots | — |
| 13 | Lightning Greaves | — |
| 14 | Path of Ancestry | — |
| 15 | Path to Exile | — |
| 16 | Counterspell | — |
| 17 | Evolving Wilds | — |
| 18 | Fellwar Stone | — |
| 19 | Rogue's Passage | — |
| 20 | Cultivate | — |
| 21 | Thought Vessel | — |
| 22 | Farseek | — |
| 23 | Blasphemous Act | — |
| 24 | Beast Within | — |
| 25 | Myriad Landscape | — |
| 26 | Rampant Growth | — |
| 27 | Bojuka Bog | — |
| 28 | Terramorphic Expanse | — |
| 29 | Chaos Warp | — |
| 30 | Nature's Lore | — |
| 31 | Mind Stone | — |
| 32 | Heroic Intervention | — |
| 33 | Birds of Paradise | 6 |
| 34 | Dark Ritual | — |
| 35 | Kodama's Reach | — |
| 36 | Solemn Simulacrum | — |
| 37 | An Offer You Can't Refuse | — |
| 38 | Polluted Delta | — |
| 39 | Commander's Sphere | — |
| 40 | Rhystic Study | — |
| 41 | Flooded Strand | — |
| 42 | Misty Rainforest | — |
| 43 | Bloodstained Mire | — |
| 44 | Skullclamp | 7c |
| 45 | Three Visits | — |
| 46 | Windswept Heath | — |
| 47 | Verdant Catacombs | — |
| 48 | Wooded Foothills | — |
| 49 | Scalding Tarn | — |
| 50 | Cyclonic Rift | — |
| 51 | Marsh Flats | — |
| 52 | Watery Grave | — |
| 53 | Negate | — |
| 54 | Arid Mesa | — |
| 55 | Generous Gift | — |

## What this report enables

For the runtime engine: when resolving continuous effects on a permanent,
the engine collects every Modification with a `layer` tag, buckets by
layer number, sorts each bucket by timestamp, and applies in layer order.
This is the §613 layer resolution algorithm, and it's now trivial because
every effect carries its layer tag at parse time.

For cage analytics: layer-7c and 7d counts tell us how many cards in the
pool do P/T modification, which correlates with tempo/aggro archetypes.
Layer-6 counts identify removal/restriction density (what interaction
exists in the format). Layer-2 counts identify how much control-change
pressure can exist in a given meta (Mind Control effects).