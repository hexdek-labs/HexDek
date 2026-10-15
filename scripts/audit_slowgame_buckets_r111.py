import os, re, collections
LOG="/home/josh/hexdek/server.log"; DECKS="/home/josh/hexdek/data/decks"
SEVEN=["chandra, hope's beacon","huatli, poet of unity","mana bloom",
       "night of the sweets' revenge","ramos, dragon engine","vivi ornitier","wall of roots"]
LOOP="sorin_of_house_markov_sorin_ravenous_neonate_b2_khazad_doom_JfUAlsri"
deck_has={}
for root,_d,files in os.walk(DECKS):
    o=os.path.basename(root)
    for fn in files:
        if fn.endswith(".txt"):
            try: b=open(os.path.join(root,fn),errors="replace").read().lower()
            except Exception: continue
            deck_has[f"{o}/{fn[:-4]}"]=any(c in b for c in SEVEN)
rx=re.compile(r"SLOW GAME ([0-9.]+)s turns=(\d+) decks=\[([^\]]*)\]")
slow=[(float(m.group(1)),int(m.group(2)),m.group(3).split())
      for L in open(LOG,errors="replace") if (m:=rx.search(L))]

# distinct participants actually scheduled into slow games
parts=collections.Counter()
for _d,_t,ds in slow:
    for d in ds: parts[d]+=1
dist=[d for d in parts if d in deck_has]
dist_with=sum(1 for d in dist if deck_has[d])
print(f"DISTINCT decks appearing in any slow game: {len(dist)}")
print(f"  of those, holding one of the seven: {dist_with}  ({100.0*dist_with/len(dist):.2f}%)")
print(f"  (pool-wide base rate was 5.18% over 1756 files)")
print()

buckets=[(0,60),(60,120),(120,240),(240,480),(480,10**9)]
print(f"{'duration':>14}  {'games':>6}  {'loopdeck':>14}  {'one-of-seven':>14}")
for lo,hi in buckets:
    sel=[g for g in slow if lo<=g[0]<hi]
    if not sel: continue
    nl=sum(1 for _d,_t,ds in sel if any(LOOP in d for d in ds))
    ns=sum(1 for _d,_t,ds in sel if any(deck_has.get(d) for d in ds))
    lab=f"{lo}-{hi}s" if hi<10**9 else f">{lo}s"
    print(f"{lab:>14}  {len(sel):6d}  {nl:5d} ({100.0*nl/len(sel):5.1f}%)  {ns:5d} ({100.0*ns/len(sel):5.1f}%)")
print()
top=sorted(slow,reverse=True)[:30]
nl=sum(1 for _d,_t,ds in top if any(LOOP in d for d in ds))
ns=sum(1 for _d,_t,ds in top if any(deck_has.get(d) for d in ds))
print(f"THIRTY WORST slow games: loopdeck in {nl}/30, one-of-seven in {ns}/30")
