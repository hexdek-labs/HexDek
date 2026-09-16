# Prepared to do — staged actions awaiting a go

Actions that are **ready to fire but deliberately not fired**. Each entry has
the exact command, the precondition, and — most importantly — **how to tell
whether it actually worked**, because several of this repo's worst bugs were
operations that reported success and changed nothing.

Nothing in this file has been run. Status lines are current as of
2026-09-15.

---

## 0. BLOCKER — backend deploy has no working path to DARKSTAR

Everything in section 1 depends on this. Reported here as the first item
because it silently gates the rest.

**Symptom**

```
$ ssh josh@192.168.1.207
josh@192.168.1.207: Permission denied (publickey,password,keyboard-interactive)
```

Also refused for `Administrator` (connection reset), `hexdek`, `darkstar`,
`admin`.

**What was established, not assumed**

| fact | evidence |
|---|---|
| The box is up and **is** prod | `http://192.168.1.207:8090/api/health` answers; uptime matches `hexdek.dev` to the second |
| Caddy points at it | MISTY `/etc/caddy/Caddyfile:295` → `reverse_proxy 192.168.1.207:8090` |
| It is **Windows** | SSH banner `OpenSSH_for_Windows_9.5` |
| Our key is offered and **refused**, not absent | `Offering public key: claude_remote_ed25519 ED25519 SHA256:lHqtfRiiOQK+pIrvWYmEN1ZHhVK52+k8v/gc5gWtiz0` → `Authentications that can continue: …` |
| MISTY is reachable | `josh@192.168.1.200` connects; Ubuntu 6.8; `~/sites/hexdek/` intact |

**Leading hypothesis.** Windows OpenSSH ignores `~/.ssh/authorized_keys`
entirely for accounts in the **Administrators** group and reads only
`C:\ProgramData\ssh\administrators_authorized_keys`. That file must also be
owned by SYSTEM/Administrators with inheritance disabled or sshd refuses it
silently. A key in the user's own folder is offered, rejected, and looks
exactly like the symptom above. This would have broken silently at the June
rebuild.

**`scripts/deploy.sh` is dead code for this host.** It cross-compiles
`GOOS=linux`, `scp`s to `$HOME/hexdek/hexdek-server`, and runs
`start-hexdek.sh` under `setsid -f`. None of that exists on Windows; it would
fail at the first `scp` even with working auth. **Do not run it against
DARKSTAR** expecting it to work.

**To unblock:** install the public key above into
`C:\ProgramData\ssh\administrators_authorized_keys` on DARKSTAR, or deploy by
hand from `origin/main`.

---

## 1. Backend deploy — STAGED, blocked on §0

**Precondition:** SSH to DARKSTAR works.

Since the deploy script targets a host shape that no longer exists, the
Windows deploy is: cross-compile for `windows/amd64`, copy to `D:\hexdek`,
restart the `HexDekServer` scheduled task.

```bash
cd /path/to/HexDek
git fetch origin && git checkout main && git pull        # expect 0687dd20 or later
GOOS=windows GOARCH=amd64 go build -o hexdek-server.exe ./cmd/hexdek-server/
GOOS=windows GOARCH=amd64 go build -o hexdek-freya.exe  ./cmd/hexdek-freya/
```

Both binaries matter: the Freya freshness work is split across the server
(`/api/.../analysis` annotation) and the Freya binary (version stamping). Ship
one without the other and the staleness banner reads the wrong thing.

> **Build from the primary clone, not a linked worktree.** Go stamps no
> `vcs.revision` from a worktree, which silently disables Freya's
> build-derived cache invalidation. Confirm before shipping:
> `./hexdek-freya --version` must print `cache mode: build-derived`.

**Verification — do not skip, and do not accept "the task restarted" as proof:**

```bash
curl -s https://hexdek.dev/api/health | python3 -m json.tool
```

| check | expected | meaning |
|---|---|---|
| `uptime_sec` | small | the process actually restarted |
| `deck_pool` key present | yes | **this is the tell** — the field only exists in the new build |
| `deck_pool.rejected` | 0, or explainable | non-zero means users are being refused |

If `deck_pool` is absent, the deploy did not take, regardless of what the
restart reported.

---

## 2. Frontend deploy — STAGED, **not blocked**

MISTY is reachable, so this can go independently of §0. Held only because the
staleness banner it renders reads fields the **current** backend does not
serve — shipping the frontend first means the banner silently renders nothing.
**Ship after §1.**

```bash
cd hexdek && npm install && VITE_API_URL="" npx vite build
rsync -av --delete hexdek/dist/ josh@192.168.1.200:~/sites/hexdek/
```

**Verification:** load a deck page and confirm the freshness banner appears
for a deck whose analysis predates the deploy. Absence is ambiguous — it means
either "everything is current" or "the field never arrived" — so check
`/api/decks/{owner}/{id}/analysis` for `stale` directly rather than trusting
the absence of a banner.

---

## 3. Freya corpus sweep — STAGED, blocked on §1

Pointless before the backend ships: it stamps versions a running server can't
read, and the banner it clears isn't rendered yet.

```bash
go build -o hexdek-freya ./cmd/hexdek-freya/
./scripts/freya-refresh-stale.sh --dry-run          # count first
./scripts/freya-refresh-stale.sh                    # then fire
```

**Measured cost, not estimated:** 1,707 decks in **23 minutes** batched, plus
~8 minutes for the stragglers `--all-decks` structurally cannot see (it skips
`benched`/`test` and globs only `*.txt`). The script re-runs those
individually and **verifies each stamp landed on disk** rather than trusting
Freya's exit code — on the first full run that check caught 23 decks the batch
had silently skipped while exiting 0 throughout.

Expected output ends `failed : 0`. Any other number names the decks.

---

## 4. CBC fold two — NOT STAGED, needs a design decision

Stage one (the wiring audit) is complete; see
`2026-09-15-silent-failure-sweep.md`. Fold two is the enforcement policy and
is deliberately **not** staged, because it needs decisions rather than a
command:

- the `additional_cost` → `[]*AdditionalCost` bridge must **parse the raw
  English clause** — the AST `args` payload is the printed sentence, not
  structured fields;
- `or` inside a cost is ambiguous: 218 cards use it for a disjunctive *object*
  ("sacrifice an artifact or creature"), 52 for a genuine choice between two
  *costs* ("sacrifice a creature or pay {3}"). One adapter cannot treat both
  the same way;
- the `Hat` interface has **no `ChoosePayUnless` seam**, and three call sites
  currently hardcode three mutually contradictory policies inline.

---

## Staging discipline

Why these sit here rather than being run:

1. **Order matters and the dependency is invisible.** Frontend-before-backend
   produces a page that renders nothing and reports no error — the same
   failure class this week was spent removing.
2. **Every entry states its own falsification.** "How do I know it worked" is
   written down *before* the action, because after the fact the temptation is
   to accept the operation's own report. Several of this repo's worst bugs
   were operations that reported success.
3. **A blocked step is listed as blocked, at the top.** The 50-hour deck-pool
   outage persisted because a failure had no way to announce itself. A staging
   list that hides its blockers repeats that at the process level.
