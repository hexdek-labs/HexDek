import re, collections
LOG="/home/josh/hexdek/server.log"
rx=re.compile(r"SLOW GAME ([0-9.]+)s turns=(\d+) decks=\[([^\]]*)\]")
slow=[(float(m.group(1)),int(m.group(2)),m.group(3).split())
      for L in open(LOG,errors="replace") if (m:=rx.search(L))]
worst=[g for g in slow if g[0]>=480]
mild =[g for g in slow if g[0]<60]
def top(games,n=10):
    c=collections.Counter()
    for _d,_t,ds in games:
        for d in ds: c[d]+=1
    return c, len(games)
cw,nw=top(worst); cm,nm=top(mild)
print(f">=480s games: {nw}   <60s games: {nm}")
print()
print(f"{'deck':<66} {'>=480s':>8} {'<60s':>8}")
for d,n in cw.most_common(12):
    print(f"{d[:64]:<66} {n:4d}/{nw:<4} {cm.get(d,0):4d}/{nm:<4}")
print()
print("turns distribution in the >=480s bucket:")
t=[g[1] for g in worst]
print(f"  n={len(t)} min={min(t)} max={max(t)} mean={sum(t)/len(t):.1f}")
tm=[g[1] for g in mild]
print(f"  <60s bucket: n={len(tm)} min={min(tm)} max={max(tm)} mean={sum(tm)/len(tm):.1f}")
print()
print("seconds-per-turn (the real pathology signal):")
for lab,gs_ in (("<60s",mild),(">=480s",worst)):
    spt=[g[0]/max(1,g[1]) for g in gs_]
    spt.sort()
    print(f"  {lab:>7}  median={spt[len(spt)//2]:7.2f}s/turn  max={spt[-1]:7.2f}s/turn")
