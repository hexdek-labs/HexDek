// mtgsquad replay viewer — vanilla JS
// -----------------------------------------------------------------------------
// The viewer is a DUMB renderer. It does not know any MTG rules. It reads
// structured events from a JSONL file produced by playloop.py --json-log and
// reconstructs per-event state by walking the event list in order.
//
// Event types we honor (additive — unknown types just render in the log):
//   header               meta about the game (not an in-game event)
//   game_start           informational
//   turn_start           advances turn counter (from event.turn field)
//   phase_change         updates current phase
//   play_land            adds a card to a seat's battlefield (untapped)
//   cast                 informational (card + cmc); hand count tracked via state
//   enter_battlefield    adds a permanent to seat's battlefield
//   resolve              instant/sorcery moves to graveyard
//   damage               informational (target info)
//   life_change          seat life: from → to
//   destroy              removes a permanent, adds card name to graveyard
//   draw                 informational
//   discard              moves hand card to graveyard
//   mill                 moves library cards to graveyard
//   state                full snapshot — our PRIMARY source of truth. Delta
//                        events are used to hint the UI between snapshots but
//                        every snapshot resets state fully. Playloop emits a
//                        snapshot at game start, end of each turn, after each
//                        cast, and at game over — so deltas rarely drift.
// -----------------------------------------------------------------------------

(function() {
  "use strict";

  // Default sample that ships next to this viewer.
  const DEFAULT_URL = "sample.jsonl";

  // --- DOM refs ---
  const $ = (id) => document.getElementById(id);
  const el = {
    fileInput: $("file-input"),
    dropHint: $("drop-hint"),
    scrubber: $("scrubber"),
    btnFirst: $("btn-first"),
    btnPrev: $("btn-prev"),
    btnNext: $("btn-next"),
    btnLast: $("btn-last"),
    seqCurrent: $("seq-current"),
    seqTotal: $("seq-total"),
    toggleAuto: $("toggle-auto"),
    speedSelect: $("speed-select"),
    eventLog: $("event-log"),
    metaGame: $("meta-game"),
    metaSeed: $("meta-seed"),
    metaTurn: $("meta-turn"),
    metaPhase: $("meta-phase"),
    metaResult: $("meta-result"),
  };
  const seatEl = (seat) => ({
    life: $(`seat-${seat}-life`),
    hand: $(`seat-${seat}-hand`),
    library: $(`seat-${seat}-library`),
    gyCount: $(`seat-${seat}-gy-count`),
    mana: $(`seat-${seat}-mana`),
    bf: $(`seat-${seat}-bf`),
    gy: $(`seat-${seat}-gy`),
  });

  // --- State machine ---
  // events[] is the raw list (excluding header, which we stash separately).
  // states[i] is the full reconstructed state AFTER event i was applied.
  // We materialize all states at load time so goto(seq) is O(1).
  let header = null;
  let events = [];
  let states = [];
  let currentSeq = 0;
  let autoplayTimer = null;

  // --- Initial empty state ---
  function emptyState() {
    return {
      turn: 0, phase: "beginning", active: 0,
      seat_0: blankSeat(),
      seat_1: blankSeat(),
      lastEventType: null,
      lastEventText: "",
      gameOver: null,
    };
  }
  function blankSeat() {
    return {
      life: 20, hand: 0, library: 0, mana_pool: 0,
      graveyard: [],
      battlefield: [],   // [{name, tapped, summoning_sick, power, toughness, damage}]
      lost: false,
    };
  }

  // --- Apply a single event as a delta, returning a NEW state ---
  function applyEvent(prev, evt) {
    const s = cloneState(prev);
    s.lastEventType = evt.type;
    s.lastEventText = describe(evt);
    if (typeof evt.turn === "number") s.turn = evt.turn;
    if (evt.phase) s.phase = evt.phase;
    if (typeof evt.seat === "number") s.active = evt.seat;

    switch (evt.type) {
      case "state": {
        // Snapshot — resets seat state completely.
        if (evt.seat_0) s.seat_0 = mergeSeatSnap(s.seat_0, evt.seat_0);
        if (evt.seat_1) s.seat_1 = mergeSeatSnap(s.seat_1, evt.seat_1);
        break;
      }
      case "life_change": {
        const seatKey = `seat_${evt.seat}`;
        if (s[seatKey] && typeof evt.to === "number") s[seatKey].life = evt.to;
        break;
      }
      case "play_land": {
        const seatKey = `seat_${evt.seat}`;
        if (s[seatKey]) {
          s[seatKey].battlefield.push({
            name: evt.card, tapped: false, summoning_sick: false,
            power: 0, toughness: 0, damage: 0,
          });
          if (s[seatKey].hand > 0) s[seatKey].hand -= 1;
        }
        break;
      }
      case "enter_battlefield": {
        const seatKey = `seat_${evt.seat}`;
        if (s[seatKey]) {
          s[seatKey].battlefield.push({
            name: evt.card, tapped: false,
            summoning_sick: !!evt.summoning_sick,
            power: 0, toughness: 0, damage: 0,
          });
          if (s[seatKey].hand > 0) s[seatKey].hand -= 1;
        }
        break;
      }
      case "cast": {
        // hand will be decremented by enter_battlefield for permanents or by
        // resolve for spells — don't double-count here.
        break;
      }
      case "resolve": {
        const seatKey = `seat_${evt.seat}`;
        if (s[seatKey] && evt.to === "graveyard") {
          s[seatKey].graveyard.push(evt.card);
          if (s[seatKey].hand > 0) s[seatKey].hand -= 1;
        }
        break;
      }
      case "destroy": {
        const seatKey = `seat_${evt.seat}`;
        if (s[seatKey]) {
          const idx = s[seatKey].battlefield.findIndex(p => p.name === evt.card);
          if (idx >= 0) s[seatKey].battlefield.splice(idx, 1);
          s[seatKey].graveyard.push(evt.card);
        }
        break;
      }
      case "discard": {
        const seatKey = `seat_${evt.seat}`;
        if (s[seatKey]) {
          s[seatKey].graveyard.push(evt.card);
          if (s[seatKey].hand > 0) s[seatKey].hand -= 1;
        }
        break;
      }
      case "mill": {
        const seatKey = `seat_${evt.seat}`;
        if (s[seatKey]) {
          const cards = evt.cards || [];
          cards.forEach(c => s[seatKey].graveyard.push(c));
          s[seatKey].library = Math.max(0, s[seatKey].library - (evt.count || cards.length));
        }
        break;
      }
      case "draw": {
        const seatKey = `seat_${evt.seat}`;
        if (s[seatKey]) {
          if (typeof evt.hand_size === "number") {
            s[seatKey].hand = evt.hand_size;
          } else {
            s[seatKey].hand += (evt.count || 1);
          }
          s[seatKey].library = Math.max(0, s[seatKey].library - (evt.count || 1));
        }
        break;
      }
      case "damage": {
        if (evt.target_kind === "permanent" && evt.target_card != null) {
          const seatKey = `seat_${evt.target_seat}`;
          const perm = s[seatKey] && s[seatKey].battlefield.find(p => p.name === evt.target_card);
          if (perm) perm.damage = (perm.damage || 0) + (evt.amount || 0);
        }
        break;
      }
      case "game_over": {
        s.gameOver = {
          winner: evt.winner, reason: evt.reason, turn: evt.turn,
        };
        break;
      }
      // game_start, turn_start, phase_change: nothing beyond the header update
      default:
        break;
    }
    return s;
  }

  function mergeSeatSnap(prev, snap) {
    return {
      life: snap.life != null ? snap.life : prev.life,
      hand: snap.hand != null ? snap.hand : prev.hand,
      library: snap.library != null ? snap.library : prev.library,
      mana_pool: snap.mana_pool != null ? snap.mana_pool : prev.mana_pool,
      graveyard: Array.isArray(snap.graveyard) ? snap.graveyard.slice() : prev.graveyard,
      battlefield: Array.isArray(snap.battlefield)
        ? snap.battlefield.map(p => ({ ...p }))
        : prev.battlefield,
      lost: !!snap.lost,
    };
  }

  function cloneState(prev) {
    return {
      turn: prev.turn, phase: prev.phase, active: prev.active,
      lastEventType: prev.lastEventType, lastEventText: prev.lastEventText,
      gameOver: prev.gameOver ? { ...prev.gameOver } : null,
      seat_0: cloneSeat(prev.seat_0),
      seat_1: cloneSeat(prev.seat_1),
    };
  }
  function cloneSeat(s) {
    return {
      life: s.life, hand: s.hand, library: s.library, mana_pool: s.mana_pool,
      lost: s.lost,
      graveyard: s.graveyard.slice(),
      battlefield: s.battlefield.map(p => ({ ...p })),
    };
  }

  // Humanize an event into a short log line.
  function describe(evt) {
    const seatLbl = typeof evt.seat === "number" ? `P${evt.seat}` : "—";
    switch (evt.type) {
      case "game_start":    return `game start (on the play: seat ${evt.on_the_play})`;
      case "turn_start":    return `turn ${evt.turn} start`;
      case "phase_change":  return `→ ${evt.phase}`;
      case "play_land":     return `${seatLbl} plays land: ${evt.card}`;
      case "cast":          return `${seatLbl} casts ${evt.card} (cmc ${evt.cmc})`;
      case "enter_battlefield": return `${evt.card} enters ${seatLbl}${evt.summoning_sick ? " (sick)" : ""}`;
      case "resolve":       return `${evt.card} → ${evt.to}`;
      case "damage": {
        const t = evt.target_kind === "player"
          ? `seat ${evt.target_seat}`
          : evt.target_card;
        return `${seatLbl} ${evt.amount} dmg → ${t}`;
      }
      case "life_change":   return `seat ${evt.seat} life ${evt.from} → ${evt.to}`;
      case "destroy":       return `destroy ${evt.card}`;
      case "discard":       return `seat ${evt.seat} discards ${evt.card}`;
      case "mill":          return `seat ${evt.seat} mills ${evt.count}`;
      case "draw":          return `seat ${evt.seat} draws ${evt.count || 1}`;
      case "state":         return `[snapshot]`;
      case "game_over":     return `game over — winner: ${evt.winner != null ? "seat " + evt.winner : "draw"}`;
      default:              return evt.type || "?";
    }
  }

  // --- Load + parse ---
  async function loadFromUrl(url) {
    try {
      const resp = await fetch(url, { cache: "no-cache" });
      if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
      const text = await resp.text();
      loadFromText(text);
    } catch (err) {
      setStatus(`failed to load ${url}: ${err.message}`, "err");
    }
  }

  function loadFromText(text) {
    const lines = text.split(/\r?\n/).filter(l => l.trim().length > 0);
    if (lines.length === 0) { setStatus("empty file", "err"); return; }

    header = null;
    events = [];
    for (const line of lines) {
      let obj;
      try { obj = JSON.parse(line); }
      catch (e) { console.warn("skip malformed line:", line); continue; }
      if (obj.type === "header") {
        header = obj;
      } else {
        events.push(obj);
      }
    }

    if (events.length === 0) { setStatus("no events in file", "err"); return; }

    // Materialize states.
    states = new Array(events.length);
    let prev = emptyState();
    for (let i = 0; i < events.length; i++) {
      prev = applyEvent(prev, events[i]);
      states[i] = prev;
    }

    // Update meta from header.
    if (header) {
      el.metaGame.textContent = `game: #${header.game_index}`;
      el.metaSeed.textContent = `seed: ${header.seed}`;
      if (header.winner != null) {
        el.metaResult.textContent = `winner: seat ${header.winner} (${header.end_reason || ""}, T${header.final_turn})`;
      } else {
        el.metaResult.textContent = header.end_reason || "in progress";
      }
    } else {
      el.metaGame.textContent = "game: —";
      el.metaSeed.textContent = "seed: —";
      el.metaResult.textContent = "";
    }

    // Configure controls.
    el.scrubber.min = 0;
    el.scrubber.max = events.length - 1;
    el.seqTotal.textContent = String(events.length - 1);

    // Render event log.
    renderEventLog();

    // Show first state.
    goto(0);
  }

  function setStatus(msg, kind) {
    el.metaResult.textContent = msg;
    el.metaResult.className = kind === "err" ? "err" : "muted";
    console[kind === "err" ? "error" : "log"](msg);
  }

  // --- Render ---
  function renderEventLog() {
    el.eventLog.innerHTML = "";
    const frag = document.createDocumentFragment();
    events.forEach((evt, i) => {
      const li = document.createElement("li");
      li.dataset.seq = String(i);
      li.classList.add(`type-${evt.type}`);
      if (typeof evt.seat === "number") li.classList.add(`seat-${evt.seat}`);
      li.innerHTML = `
        <span class="seq">${i}</span>
        <span class="turn-phase">T${evt.turn ?? "-"} [${evt.phase ?? "-"}]</span>
        <span class="body"></span>
      `;
      li.querySelector(".body").textContent = describe(evt);
      li.addEventListener("click", () => goto(i));
      frag.appendChild(li);
    });
    el.eventLog.appendChild(frag);
  }

  function renderState(s, seq) {
    // Header turn/phase/result
    el.metaTurn.textContent = `T${s.turn}`;
    el.metaPhase.textContent = `[${s.phase}]`;

    // Each seat
    renderSeat(0, s.seat_0);
    renderSeat(1, s.seat_1);

    // Scrubber / seq
    el.scrubber.value = String(seq);
    el.seqCurrent.textContent = String(seq);

    // Highlight active log entry + scroll into view.
    const prev = el.eventLog.querySelector("li.current");
    if (prev) prev.classList.remove("current");
    const cur = el.eventLog.querySelector(`li[data-seq="${seq}"]`);
    if (cur) {
      cur.classList.add("current");
      cur.scrollIntoView({ block: "nearest", inline: "nearest" });
    }

    // Button states
    el.btnFirst.disabled = seq <= 0;
    el.btnPrev.disabled = seq <= 0;
    el.btnNext.disabled = seq >= events.length - 1;
    el.btnLast.disabled = seq >= events.length - 1;
  }

  function renderSeat(idx, seat) {
    const e = seatEl(idx);
    // Flash life if it changed
    const lifeStr = String(seat.life);
    if (e.life.textContent !== lifeStr) {
      e.life.classList.remove("flash");
      // force reflow for anim restart
      void e.life.offsetWidth;
      e.life.classList.add("flash");
    }
    e.life.textContent = lifeStr;
    e.hand.textContent = String(seat.hand);
    e.library.textContent = String(seat.library);
    e.gyCount.textContent = String(seat.graveyard.length);
    e.mana.textContent = String(seat.mana_pool);

    // Battlefield
    e.bf.innerHTML = "";
    seat.battlefield.forEach(p => {
      const li = document.createElement("li");
      if (p.tapped) li.classList.add("tapped");
      if (p.summoning_sick) li.classList.add("sick");
      const pt = (p.power || p.toughness)
        ? `<span class="pt">${p.power}/${p.toughness}${p.damage ? ` (−${p.damage})` : ""}</span>`
        : "";
      li.innerHTML = `${escapeHtml(p.name)}${pt}`;
      e.bf.appendChild(li);
    });

    // Graveyard — compressed (just card names)
    e.gy.innerHTML = "";
    seat.graveyard.forEach(name => {
      const li = document.createElement("li");
      li.textContent = name;
      e.gy.appendChild(li);
    });
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"]/g, c => ({
      "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;"
    })[c]);
  }

  // --- Navigation ---
  function goto(seq) {
    if (!events.length) return;
    seq = Math.max(0, Math.min(events.length - 1, seq|0));
    currentSeq = seq;
    renderState(states[seq], seq);
  }
  function step(delta) { goto(currentSeq + delta); }

  // --- Autoplay ---
  function stopAutoplay() {
    if (autoplayTimer != null) {
      clearInterval(autoplayTimer);
      autoplayTimer = null;
    }
    el.toggleAuto.checked = false;
  }
  function startAutoplay() {
    stopAutoplay();
    const ms = parseInt(el.speedSelect.value, 10) || 350;
    autoplayTimer = setInterval(() => {
      if (currentSeq >= events.length - 1) { stopAutoplay(); return; }
      step(+1);
    }, ms);
    el.toggleAuto.checked = true;
  }

  // --- Wiring ---
  el.btnFirst.addEventListener("click", () => goto(0));
  el.btnPrev.addEventListener("click",  () => step(-1));
  el.btnNext.addEventListener("click",  () => step(+1));
  el.btnLast.addEventListener("click",  () => goto(events.length - 1));
  el.scrubber.addEventListener("input", (e) => goto(parseInt(e.target.value, 10)));
  el.toggleAuto.addEventListener("change", (e) => {
    if (e.target.checked) startAutoplay(); else stopAutoplay();
  });
  el.speedSelect.addEventListener("change", () => {
    if (autoplayTimer != null) startAutoplay();
  });

  document.addEventListener("keydown", (e) => {
    if (e.target && /INPUT|SELECT|TEXTAREA/.test(e.target.tagName)) return;
    switch (e.key) {
      case "ArrowLeft":  step(-1); e.preventDefault(); break;
      case "ArrowRight": step(+1); e.preventDefault(); break;
      case "Home":       goto(0); e.preventDefault(); break;
      case "End":        goto(events.length - 1); e.preventDefault(); break;
      case " ":
        if (autoplayTimer != null) stopAutoplay(); else startAutoplay();
        e.preventDefault();
        break;
    }
  });

  // File input
  el.fileInput.addEventListener("change", (e) => {
    const f = e.target.files && e.target.files[0];
    if (!f) return;
    readFile(f);
  });

  // Drag + drop
  function readFile(file) {
    const fr = new FileReader();
    fr.onload = () => loadFromText(String(fr.result));
    fr.onerror = () => setStatus("file read failed", "err");
    fr.readAsText(file);
  }
  ["dragenter", "dragover"].forEach(ev => {
    document.addEventListener(ev, (e) => {
      e.preventDefault(); e.stopPropagation();
      document.body.classList.add("dragging");
    });
  });
  ["dragleave", "drop"].forEach(ev => {
    document.addEventListener(ev, (e) => {
      if (ev === "dragleave" && e.target !== document.body) return;
      e.preventDefault(); e.stopPropagation();
      document.body.classList.remove("dragging");
    });
  });
  document.addEventListener("drop", (e) => {
    const f = e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0];
    if (f) readFile(f);
  });

  // --- Boot: try to load the bundled sample ---
  loadFromUrl(DEFAULT_URL);
})();
