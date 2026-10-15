# r120 prototype: does a comment's QUOTED rule text match the CR text of the
# rule number it cites? The text-lock ratchet cannot see this class: it pins
# "cited rule -> CR text" and stays green when the CODE's number shifts under
# a stable meaning.
import re, sys, pathlib

crpath, root = sys.argv[1], sys.argv[2]
cr = {}
for m in re.finditer(r"^(\d{3}\.\d+[a-z]*)\.?\s+(.*)$", open(crpath).read(), re.M):
    cr[m.group(1)] = m.group(2)

def norm(s):
    s = s.lower()
    s = s.replace("’", "'").replace("‘", "'")
    s = s.replace("“", '"').replace("”", '"')
    s = s.replace("—", "-").replace("–", "-")
    return re.sub(r"[^a-z0-9]+", " ", s).strip()

RULE = re.compile(r"§?(\d{3}\.\d+[a-z]+)")
NCR = {r: norm(t) for r, t in cr.items()}
hits = 0
for f in sorted(pathlib.Path(root).rglob("*.go")):
    if f.name.endswith("_test.go"):
        continue
    lines = open(f).read().split("\n")
    i = 0
    while i < len(lines):
        if not lines[i].lstrip().startswith("//"):
            i += 1; continue
        j = i
        while j < len(lines) and lines[j].lstrip().startswith("//"):
            j += 1
        block = " ".join(l.lstrip().lstrip("/").strip() for l in lines[i:j])
        # quoted spans of real length, straight or typographic
        for q in re.findall(r'"([^"]{40,400})"', block):
            rules = RULE.findall(block)
            if not rules:
                continue
            nq = norm(q)
            if any(nq in NCR.get(r, "") for r in rules):
                continue
            # which rule DOES contain it?
            owner = [r for r, t in NCR.items() if nq and nq in t]
            if owner:
                hits += 1
                print(f"{f.relative_to(root)}:{i+1}  cites {rules[:3]}  but the quoted text is {owner[:3]}")
        i = j
print(f"\nquoted-text/number mismatches: {hits}")
