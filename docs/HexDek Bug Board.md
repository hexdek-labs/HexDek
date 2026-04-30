---

kanban-plugin: board

---

## Known Unknowns

- [ ] Copy token mana value — do copy tokens (Clone, Satya, Populate) inherit mana_cost from the original? CR 706.2 says copiable values include mana cost. If engine zeros it on token creation, mana-value-matters cards (Engineered Explosives, Culling Sun, etc.) break. Needs verification. #engine #copy


## Confirmed Bugs



## Under Investigation



## Fixed

- [x] Tergrid recursive trigger crash (depth guard + total trigger cap) #engine
- [x] Obeka wrong ability resolution #engine
- [x] DFC commander name mismatch #engine
- [x] Compound type filter for cast triggers #engine
- [x] 8 dead per_card triggers — 7 fixed, alias normalization added #engine
- [x] Freya false positives (~20/28) — self-exile, hand vs battlefield, attack-trigger dependency, randomness #freya



%% kanban:settings
```
{"kanban-plugin":"board","list-collapse":[false,false,false,false]}
```
%%
