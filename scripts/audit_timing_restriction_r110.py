import json, collections
PATH="/home/josh/hexdek/data/rules/ast_dataset.jsonl"
static_nodes=collections.Counter()   # args text -> count
static_cards=set()
field_set=set()                      # cards where Activated.timing_restriction is non-null
field_vals=collections.Counter()
zero_cost_mana=set()                 # the dangerous subset

def walk(n, out):
    if isinstance(n, dict):
        if n.get("__ast_type__")=="Modification" and n.get("kind")=="timing_restriction":
            a=n.get("args") or []
            out["static"].append(a[0] if a and isinstance(a[0],str) else "")
        if n.get("__ast_type__")=="Activated":
            tr=n.get("timing_restriction")
            if tr: out["field"].append(str(tr))
            # free mana ability?
            cost=n.get("cost") or {}
            mana=cost.get("mana") or {}
            syms=mana.get("symbols") or []
            free = (not cost.get("tap")) and (not cost.get("sacrifice")) and \
                   (not cost.get("pay_life")) and (not cost.get("discard")) and \
                   (not cost.get("remove_counters")) and \
                   all((s.get("generic")==0 and not s.get("color")) for s in syms)
            eff=n.get("effect") or {}
            if free and isinstance(eff,dict) and str(eff.get("kind","")).startswith("add_mana"):
                out["freemana"].append(1)
        for v in n.values(): walk(v,out)
    elif isinstance(n,list):
        for v in n: walk(v,out)

tot=0
for line in open(PATH):
    try: e=json.loads(line)
    except Exception: continue
    tot+=1
    out={"static":[],"field":[],"freemana":[]}
    walk(e.get("ast"), out)
    nm=e.get("name","?")
    if out["static"]:
        static_cards.add(nm)
        for s in out["static"]: static_nodes[s]+=1
    if out["field"]:
        field_set.add(nm)
        for f in out["field"]: field_vals[f]+=1
    if out["freemana"] and out["static"]:
        zero_cost_mana.add(nm)

print("AST entries:", tot)
print()
print("cards with a Static timing_restriction MODIFICATION node:", len(static_cards))
print("  (this is the form the resolve handler no-ops and legality never sees)")
for t,c in static_nodes.most_common(15): print(f"   {c:5d}  {t!r}")
print()
print("cards with Activated.timing_restriction FIELD set:", len(field_set))
print("  (this is the form legality.go:1110 actually reads)")
for t,c in field_vals.most_common(10): print(f"   {c:5d}  {t!r}")
print()
print("OVERLAP (both forms):", len(static_cards & field_set))
print()
print("DANGEROUS SUBSET — free-cost mana ability + a Static timing_restriction:", len(zero_cost_mana))
for n in sorted(zero_cost_mana)[:20]: print("   ", n)
