// mtgsquad web client — vanilla JS, single-page

(function () {
  'use strict';

  // ---------- state ----------
  const state = {
    deviceId: null,
    sessionToken: null,
    deckId: null,
    partyId: null,
    gameId: null,
    seat: null,
    ws: null,
    selectedCardId: null,
    snapshot: null,
  };

  // ---------- persistence ----------
  // Identity (device id + session token) is stored in BOTH localStorage and
  // a cookie. localStorage is the primary (same-origin, persists across
  // sessions); the cookie is a hedge in case localStorage is cleared or
  // disabled (e.g. private browsing). Same-site cookie so no CSRF risk.
  const IDENTITY_KEY = 'mtgsquad.identity';
  const COOKIE_KEY = 'mtgsquad_identity';
  const COOKIE_TTL_DAYS = 30;

  function saveIdentity(deviceId, sessionToken, displayName, deckId) {
    const existing = loadIdentity() || {};
    const payload = {
      deviceId: deviceId ?? existing.deviceId,
      sessionToken: sessionToken ?? existing.sessionToken,
      displayName: displayName ?? existing.displayName,
      deckId: deckId !== undefined ? deckId : existing.deckId,
      savedAt: Date.now(),
    };
    try { localStorage.setItem(IDENTITY_KEY, JSON.stringify(payload)); } catch {}
    try {
      const exp = new Date(Date.now() + COOKIE_TTL_DAYS * 86400e3).toUTCString();
      document.cookie = `${COOKIE_KEY}=${encodeURIComponent(JSON.stringify(payload))}; path=/; expires=${exp}; SameSite=Lax`;
    } catch {}
  }

  function loadIdentity() {
    try {
      const raw = localStorage.getItem(IDENTITY_KEY);
      if (raw) return JSON.parse(raw);
    } catch {}
    // Cookie fallback
    const match = document.cookie.match(new RegExp('(^| )' + COOKIE_KEY + '=([^;]+)'));
    if (match) {
      try { return JSON.parse(decodeURIComponent(match[2])); } catch {}
    }
    return null;
  }

  function clearIdentity() {
    try { localStorage.removeItem(IDENTITY_KEY); } catch {}
    document.cookie = `${COOKIE_KEY}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Lax`;
  }

  // ---------- DOM ----------
  const $ = (id) => document.getElementById(id);
  const setupPanel = $('setup-panel');
  const gamePanel = $('game-panel');
  const statusPill = $('status-pill');
  const partyCodeEl = $('party-code');

  // ---------- HTTP helpers ----------
  async function postJSON(path, body) {
    const res = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : null,
    });
    const text = await res.text();
    if (!res.ok) throw new Error(`POST ${path} ${res.status}: ${text}`);
    return text ? JSON.parse(text) : null;
  }

  async function getJSON(path) {
    const res = await fetch(path);
    if (!res.ok) throw new Error(`GET ${path} ${res.status}`);
    return res.json();
  }

  // ---------- card oracle (lazy-loaded image cache) ----------
  const cardCache = new Map();
  async function getCardArt(name) {
    if (cardCache.has(name)) return cardCache.get(name);
    try {
      const c = await getJSON(`/api/oracle/card/${encodeURIComponent(name)}`);
      cardCache.set(name, c.image_uri_art || c.image_uri_normal || '');
      return cardCache.get(name);
    } catch {
      cardCache.set(name, '');
      return '';
    }
  }

  // ---------- WebSocket ----------
  function connectWS() {
    if (state.ws) state.ws.close();
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${location.host}/ws/party/${state.partyId}?token=${state.sessionToken}`;
    state.ws = new WebSocket(url);
    state.ws.onopen = () => {
      statusPill.textContent = 'connected';
      statusPill.className = 'status-pill status-connected';
      logEvent('WebSocket connected');
    };
    state.ws.onclose = () => {
      statusPill.textContent = 'disconnected';
      statusPill.className = 'status-pill status-disconnected';
      logEvent('WebSocket closed');
    };
    state.ws.onerror = (e) => {
      logEvent('WS error: ' + (e.message || 'unknown'));
    };
    state.ws.onmessage = handleWSMessage;
  }

  function send(type, payload) {
    if (!state.ws || state.ws.readyState !== WebSocket.OPEN) {
      logEvent('WS not open, cannot send ' + type);
      return;
    }
    state.ws.send(JSON.stringify({ type, payload: payload || null }));
  }

  function handleWSMessage(evt) {
    let msg;
    try { msg = JSON.parse(evt.data); } catch { return; }
    switch (msg.type) {
      case 'welcome':
        logEvent(`welcomed (party: ${msg.payload.party_id}, ${msg.payload.connected_devices.length} devices connected)`);
        break;
      case 'device_joined':
        logEvent(`device joined: ${msg.payload.device_id.slice(0, 8)}`);
        break;
      case 'game.state_update':
        applySnapshot(msg.payload);
        break;
      case 'game.yuriko_triggered':
        logEvent(`★ Yuriko reveal: ${msg.payload.revealed_card} (${msg.payload.revealed_cmc} dmg) → seat ${msg.payload.target_seat}`);
        break;
      case 'pong':
        logEvent('pong');
        break;
      case 'chat':
        logEvent(`chat from ${msg.payload.from_device.slice(0, 8)}: ${msg.payload.text}`);
        break;
      case 'error':
        logEvent('ERROR: ' + msg.payload.message);
        break;
      default:
        logEvent('unknown msg type: ' + msg.type);
    }
  }

  // ---------- snapshot rendering ----------
  function applySnapshot(snap) {
    state.snapshot = snap;
    state.gameId = snap.game.id;
    if (snap.you) state.seat = snap.you.seat_position;

    setupPanel.classList.add('hidden');
    gamePanel.classList.remove('hidden');

    // Turn / phase / active
    if (snap.turn) {
      $('turn-number').textContent = snap.turn.turn_number;
      $('phase').textContent = snap.turn.phase;
      $('active-seat').textContent = snap.turn.active_seat === state.seat ? 'YOU' : `seat ${snap.turn.active_seat}`;
    }

    // ---- Your zone ----
    const you = snap.you;
    const bfBySeat = snap.battlefield_by_seat || {};
    const cmdBySeat = snap.command_by_seat || {};
    const gyBySeat = snap.graveyard_by_seat || {};
    const yourBF = bfBySeat[state.seat] || [];

    if (you) {
      $('your-life').textContent = you.life;
      // Floating mana pool (mana already produced this turn but not spent)
      $('mana-w').textContent = you.mana_pool_w || 0;
      $('mana-u').textContent = you.mana_pool_u || 0;
      $('mana-b').textContent = you.mana_pool_b || 0;
      $('mana-r').textContent = you.mana_pool_r || 0;
      $('mana-g').textContent = you.mana_pool_g || 0;
      $('mana-c').textContent = you.mana_pool_c || 0;
    }
    $('your-library').textContent = snap.your_library_size || 0;
    $('your-hand-count').textContent = (snap.your_hand || []).length;
    $('your-gy').textContent = (snap.your_graveyard || []).length;

    // Available mana = untapped lands by inferred color
    renderAvailableMana($('your-available-mana'), yourBF);

    // Your zones — split battlefield into lands / non-land permanents (playmat style)
    const yourLands = yourBF.filter(c => c && (c.types || []).includes('Land'));
    const yourPerms = yourBF.filter(c => c && !(c.types || []).includes('Land'));
    renderCards($('your-command'), cmdBySeat[state.seat] || [], true, false, true);
    renderCards($('your-permanents'), yourPerms, true, false);
    renderCards($('your-lands'), yourLands, true, false);
    renderCards($('your-hand'), snap.your_hand || [], true, true);

    // ---- Opponents zone ----
    const oppZone = $('opp-zone');
    oppZone.innerHTML = '';
    const opponents = snap.opponents || [];
    if (!opponents.length) {
      oppZone.innerHTML = '<div class="muted" style="padding:12px;">no opponents in this game</div>';
    }
    opponents.forEach(opp => {
      const seat = opp.seat_position;
      const handSize = (snap.opp_hand_sizes || {})[seat] || 0;
      const libSize = (snap.opp_library_sizes || {})[seat] || 0;
      const oppBF = bfBySeat[seat] || [];
      const oppCmd = cmdBySeat[seat] || [];
      const oppGY = gyBySeat[seat] || [];
      const isActive = snap.turn && snap.turn.active_seat === seat;

      const panel = document.createElement('div');
      panel.className = 'opp-panel' + (isActive ? ' active' : '');
      panel.innerHTML = `
        <div class="seat-meta">
          <div class="seat-name">SEAT ${seat}</div>
          <div class="seat-life">${opp.life}</div>
          <div class="seat-counters">
            <span class="counter-pill"><span class="lbl">lib</span><span>${libSize}</span></span>
            <span class="counter-pill"><span class="lbl">hand</span><span>${handSize}</span></span>
            <span class="counter-pill"><span class="lbl">gy</span><span>${oppGY.length}</span></span>
          </div>
          <div>
            <div class="lbl-tiny">hand</div>
            <div class="hand-backs"></div>
          </div>
          <div>
            <div class="lbl-tiny">library</div>
            <div class="lib-spine"></div>
          </div>
        </div>
        <div class="opp-zones">
          <div class="zone-row">
            <div class="zone-label">cmd</div>
            <div class="zone-cards small-zone opp-cmd"></div>
          </div>
          <div class="zone-row">
            <div class="zone-label">field</div>
            <div class="zone-cards opp-bf"></div>
          </div>
          <div class="zone-row">
            <div class="zone-label">gy</div>
            <div class="zone-cards small-zone opp-gy"></div>
          </div>
        </div>
      `;
      // Render hand backs (cap at 10 visible, then "+N" overflow)
      const backsEl = panel.querySelector('.hand-backs');
      const visibleBacks = Math.min(handSize, 10);
      for (let i = 0; i < visibleBacks; i++) {
        const b = document.createElement('span');
        b.className = 'hand-back';
        backsEl.appendChild(b);
      }
      if (handSize > 10) {
        const o = document.createElement('span');
        o.className = 'hand-back-overflow';
        o.textContent = `+${handSize - 10}`;
        backsEl.appendChild(o);
      }
      // Render library spine (single bar with height proportional to deck size, capped)
      const spineEl = panel.querySelector('.lib-spine');
      const spineHeight = Math.min(40, Math.max(4, Math.round(libSize * 0.4)));
      const bar = document.createElement('span');
      bar.className = 'spine-bar';
      bar.style.height = spineHeight + 'px';
      spineEl.appendChild(bar);
      const count = document.createElement('span');
      count.className = 'spine-count';
      count.textContent = libSize;
      spineEl.appendChild(count);

      // Render their public zones — split battlefield into lands / non-lands like the playmat
      const oppLands = oppBF.filter(c => c && (c.types || []).includes('Land'));
      const oppPerms = oppBF.filter(c => c && !(c.types || []).includes('Land'));
      renderCards(panel.querySelector('.opp-cmd'), oppCmd, false, false, true);
      const bfEl = panel.querySelector('.opp-bf');
      bfEl.innerHTML = '';
      // Stack non-lands on top, lands on bottom — matches playmat orientation when opp panel is mirrored
      const permsRow = document.createElement('div');
      permsRow.className = 'mini-row';
      const landsRow = document.createElement('div');
      landsRow.className = 'mini-row';
      bfEl.appendChild(permsRow);
      bfEl.appendChild(landsRow);
      renderCards(permsRow, oppPerms, false, false, true);
      renderCards(landsRow, oppLands, false, false, true);
      renderCards(panel.querySelector('.opp-gy'), oppGY, false, false, true);

      oppZone.appendChild(panel);
    });
  }

  // Available-mana view: count untapped lands on your battlefield, infer color
  // by basic-land subtype. Multi-color lands show as "?" (player picks at tap time).
  function renderAvailableMana(container, battlefield) {
    container.innerHTML = '';
    const counts = { W: 0, U: 0, B: 0, R: 0, G: 0, C: 0, '?': 0 };
    for (const card of battlefield) {
      if (!card || card.tapped) continue;
      const types = card.types || [];
      if (!types.includes('Land')) continue;
      const subs = card.subtypes || [];
      let inferred = null;
      const colorMap = { Plains: 'W', Island: 'U', Swamp: 'B', Mountain: 'R', Forest: 'G' };
      const matched = subs.map(s => colorMap[s]).filter(Boolean);
      if (matched.length === 1) inferred = matched[0];
      else if (matched.length > 1) inferred = '?';
      else inferred = 'C'; // colorless if no basic subtype matched
      counts[inferred]++;
    }
    const order = ['W', 'U', 'B', 'R', 'G', 'C', '?'];
    let any = false;
    for (const color of order) {
      const n = counts[color];
      if (n === 0) continue;
      any = true;
      for (let i = 0; i < n; i++) {
        const pip = document.createElement('span');
        pip.className = 'mana-pip ' + (color === '?' ? 'c' : color.toLowerCase());
        pip.textContent = color;
        container.appendChild(pip);
      }
    }
    if (!any) {
      const empty = document.createElement('span');
      empty.className = 'lbl-tiny';
      empty.textContent = '— no untapped lands';
      container.appendChild(empty);
    }
  }

  // renderCards(container, cards, owned, isHand, readonly?)
  //   owned    — these are your cards (enables play/tap/cast handlers)
  //   isHand   — cards are in hand (changes click semantics)
  //   readonly — never wire click handlers (e.g. opponent zones, command zone view)
  function renderCards(container, cards, owned, isHand, readonly) {
    container.innerHTML = '';
    cards.filter(Boolean).forEach(card => {
      const chip = document.createElement('div');
      chip.className = 'card-chip';
      if (card.tapped) chip.classList.add('tapped');
      if (state.selectedCardId === card.instance_id) chip.classList.add('selected');

      const types = (card.types || []).join(' ');
      chip.innerHTML = `
        <img class="card-art" alt="" src="" data-name="${escapeHtml(card.name)}">
        <div class="card-name">${escapeHtml(card.name)}</div>
        <div class="card-cost">${escapeHtml(card.mana_cost || '')} · ${escapeHtml(types)}</div>
      `;

      const img = chip.querySelector('img');
      getCardArt(card.name).then(url => { if (url) img.src = url; });

      if (readonly) {
        chip.style.cursor = 'default';
        container.appendChild(chip);
        return;
      }

      chip.addEventListener('click', (ev) => {
        ev.stopPropagation();
        state.selectedCardId = card.instance_id;

        if (!owned) {
          // Opponent card — selection only, no actions
          applySnapshot(state.snapshot);
          return;
        }
        if (isHand && card.types && card.types.includes('Land')) {
          send('game.play_land', { instance_id: card.instance_id });
        } else if (!isHand && card.types && card.types.includes('Land')) {
          // Toggle tap/untap on click
          if (card.tapped) {
            send('game.untap_card', { instance_id: card.instance_id });
          } else {
            send('game.tap_land', { instance_id: card.instance_id });
          }
        } else if (!isHand && !(card.types || []).includes('Land')) {
          // Non-land permanent on battlefield → toggle tap/untap (e.g. creatures attacking)
          if (card.tapped) {
            send('game.untap_card', { instance_id: card.instance_id });
          } else {
            send('game.tap_card', { instance_id: card.instance_id });
          }
        } else if (isHand && !(card.types || []).includes('Land')) {
          send('game.cast_spell', { instance_id: card.instance_id, x_value: 0 });
        }
        applySnapshot(state.snapshot); // optimistic re-render for selection feedback
      });

      container.appendChild(chip);
    });
  }

  function escapeHtml(s) {
    return (s || '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  }

  function logEvent(msg) {
    const li = document.createElement('li');
    const time = new Date().toLocaleTimeString();
    li.textContent = `[${time}] ${msg}`;
    $('event-log').prepend(li);
  }

  // ---------- setup actions ----------
  $('btn-register').addEventListener('click', async () => {
    try {
      const name = $('display-name').value || 'Player';
      const r = await postJSON('/api/device/register', { display_name: name });
      state.deviceId = r.device.ID;
      state.sessionToken = r.session.Token;
      saveIdentity(state.deviceId, state.sessionToken, name);
      $('device-info').textContent = `device id: ${state.deviceId} (sticky)`;
      logEvent('registered ' + state.deviceId.slice(0, 8) + ' (saved to local storage)');
      loadMyDecks();
    } catch (e) { logEvent(e.message); }
  });

  // "Forget device" button — clears saved identity and reloads for a fresh start.
  // Added dynamically so we don't need to modify index.html.
  (function wireForgetButton() {
    const info = $('device-info');
    const btn = document.createElement('button');
    btn.textContent = 'Forget device';
    btn.style.cssText = 'margin-left:8px; padding:2px 8px; font-size:10px; background:transparent; color:var(--text-muted); border:1px solid var(--border);';
    btn.addEventListener('click', () => {
      if (!confirm('Clear saved device and session? You will need to register again.')) return;
      clearIdentity();
      location.reload();
    });
    info.parentElement.appendChild(btn);
  })();

  // ---------- deck tabs ----------
  document.querySelectorAll('.deck-tab').forEach(btn => {
    btn.addEventListener('click', () => {
      const tab = btn.dataset.tab;
      document.querySelectorAll('.deck-tab').forEach(b => b.classList.toggle('active', b === btn));
      document.querySelectorAll('.deck-tab-panel').forEach(p => {
        p.classList.toggle('hidden', p.dataset.panel !== tab);
      });
      if (tab === 'mine' && state.deviceId) loadMyDecks();
    });
  });

  async function loadPremades() {
    try {
      const list = await getJSON('/api/decks/premade');
      const sel = $('premade-select');
      const aiSel = $('ai-deck-select');
      sel.innerHTML = '';
      aiSel.innerHTML = '<option value="">Hex (AI) — Yuriko default</option>';
      if (!list.length) {
        sel.innerHTML = '<option value="">(no premade decks)</option>';
        return;
      }
      list.forEach(p => {
        const opt = document.createElement('option');
        opt.value = p.file;
        opt.textContent = `${p.name} — ${p.commander} (${p.cards})`;
        opt.dataset.name = p.name;
        sel.appendChild(opt);

        const aiOpt = document.createElement('option');
        aiOpt.value = p.file;
        aiOpt.textContent = `Hex → ${p.commander}`;
        aiSel.appendChild(aiOpt);
      });
    } catch (e) { logEvent('load premades: ' + e.message); }
  }

  async function loadMyDecks() {
    if (!state.deviceId) return;
    try {
      const decks = await getJSON(`/api/device/${state.deviceId}/decks`);
      const list = $('mine-list');
      if (!decks.length) {
        list.innerHTML = '<p class="muted">No decks yet. Import one via Premade or Paste.</p>';
        return;
      }
      // If no deck is currently selected and we have at least one, auto-pick
      // the most recent so "create party" always has something to set_deck with.
      if (!state.deckId && decks.length) {
        state.deckId = decks[0].ID;
        saveIdentity(null, null, null, state.deckId);
        $('deck-info').textContent = `auto-selected: ${decks[0].Name} (${decks[0].ID.slice(0, 8)}…)`;
      }
      list.innerHTML = '';
      decks.forEach(d => {
        const row = document.createElement('div');
        row.className = 'mine-item' + (state.deckId === d.ID ? ' selected' : '');
        row.innerHTML = `
          <div class="mi-name">${escapeHtml(d.Name)}</div>
          <div class="mi-meta">${escapeHtml(d.CommanderName || '-')} · ${escapeHtml(d.Format || 'commander')} · imported ${new Date(d.ImportedAt * 1000).toLocaleString()}</div>
          <div class="mi-actions">
            <button class="btn-select">Select</button>
            <button class="btn-promote">Save as premade</button>
          </div>
        `;
        row.querySelector('.btn-select').addEventListener('click', async (ev) => {
          ev.stopPropagation();
          state.deckId = d.ID;
          saveIdentity(null, null, null, d.ID);
          $('deck-info').textContent = `selected: ${d.Name} (${d.ID.slice(0, 8)}…)`;
          logEvent('selected deck ' + d.Name);
          // If we're already in a party, sync this selection to the server so
          // "start game" knows which deck our seat is running.
          if (state.partyId) {
            try {
              await postJSON(`/api/party/${state.partyId}/set_deck`, { device_id: state.deviceId, deck_id: d.ID });
              logEvent('deck synced to party');
            } catch (e) { logEvent('set_deck: ' + e.message); }
          }
          loadMyDecks();
        });
        row.querySelector('.btn-promote').addEventListener('click', async (ev) => {
          ev.stopPropagation();
          try {
            const r = await postJSON(`/api/deck/${d.ID}/save_as_premade`, {});
            logEvent(`saved as premade: ${r.filename}`);
            loadPremades();
          } catch (e) { logEvent('promote: ' + e.message); }
        });
        list.appendChild(row);
      });
    } catch (e) { logEvent('load my decks: ' + e.message); }
  }

  $('btn-refresh-mine').addEventListener('click', loadMyDecks);

  $('btn-import-premade').addEventListener('click', async () => {
    if (!state.deviceId) return alert('Register first');
    const sel = $('premade-select');
    const file = sel.value;
    if (!file) return;
    const displayName = sel.selectedOptions[0]?.dataset.name || file;
    try {
      const deckJSON = await fetch(`/api/decks/premade/${encodeURIComponent(file)}`).then(r => {
        if (!r.ok) throw new Error(`premade ${file} not reachable`);
        return r.text();
      });
      const deck = JSON.parse(deckJSON);
      const r = await postJSON('/api/deck/import', {
        owner_device_id: state.deviceId,
        name: displayName,
        deck: deck,
      });
      state.deckId = r.ID;
      saveIdentity(null, null, null, r.ID);
      $('deck-info').textContent = `imported "${displayName}" → ${r.ID.slice(0, 12)}…`;
      logEvent('imported premade ' + displayName);
    } catch (e) { logEvent('premade import: ' + e.message); }
  });

  $('btn-import-paste').addEventListener('click', async () => {
    if (!state.deviceId) return alert('Register first');
    const decklist = $('paste-decklist').value;
    const name = $('paste-name').value.trim();
    const url = $('paste-url').value.trim();
    if (!decklist.trim()) return alert('Paste a decklist first');
    const btn = $('btn-import-paste');
    btn.disabled = true;
    const prevText = btn.textContent;
    btn.textContent = 'Resolving cards…';
    try {
      const r = await postJSON('/api/deck/import_decklist', {
        owner_device_id: state.deviceId,
        name: name || undefined,
        moxfield_url: url || undefined,
        decklist,
      });
      state.deckId = r.ID;
      saveIdentity(null, null, null, r.ID);
      const msg = `imported "${r.Name}" (${r.card_count} cards` +
        (r.unresolved?.length ? `, ${r.unresolved.length} unresolved` : '') + `) → ${r.ID.slice(0, 8)}…`;
      $('deck-info').textContent = msg;
      logEvent(msg);
      if (r.unresolved?.length) {
        logEvent('unresolved: ' + r.unresolved.slice(0, 5).join(', ') + (r.unresolved.length > 5 ? '…' : ''));
      }
      $('paste-decklist').value = '';
      $('paste-name').value = '';
      $('paste-url').value = '';
    } catch (e) { logEvent('paste import: ' + e.message); }
    finally { btn.disabled = false; btn.textContent = prevText; }
  });

  $('btn-create-party').addEventListener('click', async () => {
    if (!state.deviceId) return alert('Register first');
    try {
      const r = await postJSON('/api/party/create', { host_device_id: state.deviceId, max_players: 4 });
      state.partyId = r.ID;
      partyCodeEl.textContent = state.partyId;
      $('party-info').textContent = `party code: ${state.partyId} (share with friend)`;
      if (state.deckId) {
        await postJSON(`/api/party/${state.partyId}/set_deck`, { device_id: state.deviceId, deck_id: state.deckId });
      }
      $('btn-start-game').disabled = false;
      connectWS();
    } catch (e) { logEvent(e.message); }
  });

  $('btn-join-party').addEventListener('click', async () => {
    const code = $('join-code').value.toUpperCase();
    if (!code || !state.deviceId || !state.deckId) return alert('Register and import a deck first');
    try {
      await postJSON(`/api/party/${code}/join`, { device_id: state.deviceId, deck_id: state.deckId });
      state.partyId = code;
      partyCodeEl.textContent = code;
      $('party-info').textContent = `joined ${code}`;
      $('btn-start-game').disabled = false;
      connectWS();
    } catch (e) { logEvent(e.message); }
  });

  $('btn-add-ai').addEventListener('click', async () => {
    if (!state.partyId) return alert('Create or join a party first');
    const deckFile = $('ai-deck-select').value || '';
    const btn = $('btn-add-ai');
    btn.disabled = true;
    try {
      const body = deckFile ? { deck_file: deckFile } : {};
      const r = await postJSON(`/api/party/${state.partyId}/add_ai`, body);
      logEvent(`added AI: ${r.display_name} → ${r.commander_name}`);
      $('party-members').textContent = `AI joined at seat ${r.SeatPosition}`;
    } catch (e) { logEvent('add ai: ' + e.message); }
    finally { btn.disabled = false; }
  });

  $('btn-start-game').addEventListener('click', async () => {
    if (!state.partyId) return;
    try {
      // Safety net: make sure our seat has a deck before asking the server to
      // start. Covers the case where the party was created before deckId was
      // set, or where a refresh lost partyId sync.
      if (state.deckId) {
        try {
          await postJSON(`/api/party/${state.partyId}/set_deck`, { device_id: state.deviceId, deck_id: state.deckId });
        } catch (e) { /* already set is fine */ }
      } else {
        alert('Select or import a deck before starting.');
        return;
      }
      const r = await postJSON(`/api/party/${state.partyId}/start_game`, null);
      state.gameId = r.id;
      logEvent('game started: ' + state.gameId.slice(0, 8));
      // request snapshot
      setTimeout(() => send('game.snapshot', null), 200);
    } catch (e) { logEvent(e.message); }
  });

  // ---------- game actions ----------
  $('btn-advance-phase').addEventListener('click', () => send('game.advance_phase', null));
  $('btn-draw').addEventListener('click', () => send('game.draw', { count: 1 }));
  $('btn-untap-all').addEventListener('click', () => send('game.untap_all', null));
  $('btn-yuriko-reveal').addEventListener('click', () => {
    if (!state.snapshot || !state.snapshot.opponents || !state.snapshot.opponents.length) return;
    send('game.yuriko_reveal', { target_seat: state.snapshot.opponents[0].seat_position });
  });
  $('btn-snapshot').addEventListener('click', () => send('game.snapshot', null));

  // Life ± buttons
  document.querySelectorAll('.life-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const delta = parseInt(btn.dataset.delta, 10) || 0;
      send('game.adjust_life', { delta });
    });
  });

  // ---------- bootstrap ----------
  logEvent('mtgsquad client loaded');
  loadPremades();

  // Restore saved identity if present. We validate the device still exists
  // on the server (DB may have been wiped) before trusting the cached id.
  (async function restoreIdentity() {
    const saved = loadIdentity();
    if (!saved || !saved.deviceId || !saved.sessionToken) return;
    try {
      const d = await getJSON(`/api/device/${saved.deviceId}`);
      if (!d || !d.ID) throw new Error('device missing');
      state.deviceId = saved.deviceId;
      state.sessionToken = saved.sessionToken;
      if (saved.deckId) state.deckId = saved.deckId;
      if (saved.displayName) $('display-name').value = saved.displayName;
      $('device-info').textContent = `device id: ${state.deviceId} (restored from local storage)`;
      logEvent('restored device ' + state.deviceId.slice(0, 8) + (saved.deckId ? ' + deck ' + saved.deckId.slice(0, 8) : ''));
      loadMyDecks();
    } catch {
      // Server doesn't recognise this device (DB wiped or mismatched) — clear
      // stale identity so the user registers fresh.
      clearIdentity();
      logEvent('saved identity stale, cleared');
    }
  })();

})();
