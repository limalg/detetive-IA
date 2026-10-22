(() => {
  const $ = (id) => document.getElementById(id);
  const qs = new URLSearchParams(location.search);
  const API = qs.get('api') || '';
  const DEMO = qs.has('demo');
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const brl = (n) => '$ ' + Number(n).toLocaleString('pt-BR');
  const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const COLORS = ['#2f6bff', '#22c55e', '#facc15', '#ef4444', '#a855f7', '#f97316'];
  const VOTE_SECS = 15;

  let roomId = null, meId = null, raw = null, state = null, busy = false, polling = false;
  let localVoting = null, myVote = -1, closing = false, starting = false, lastTurn = null, boardBuilt = false;
  const pawns = new Map(), prevPos = new Map();

  // ---------- API ----------
  async function api(path, body, signal) {
    if (DEMO) return demoApi(path, body);
    const res = await fetch(API + path, {
      method: body ? 'POST' : 'GET',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
      credentials: 'include',
      signal,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || data.message || 'Erro ' + res.status);
    return data;
  }
  const getRoom = (id) => api('/api/banco3d/room/state?room_id=' + encodeURIComponent(id));

  // saved player id per room, so a reload reconnects instead of re-joining
  const saveKey = (id) => 'banco3d:' + id;
  const loadSaved = (id) => { try { return JSON.parse(localStorage.getItem(saveKey(id))); } catch { return null; } };
  const forget = (id) => { try { localStorage.removeItem(saveKey(id)); if (localStorage.getItem('banco3d:last') === id) localStorage.removeItem('banco3d:last'); } catch { /* private mode */ } };
  const save = (id, v) => { try { localStorage.setItem(saveKey(id), JSON.stringify(v)); } catch { /* private mode */ } };

  // ---------- backend shape -> view shape ----------
  const TT = { GO: 'START', CHANCE: 'LUCK', CHEST: 'LUCK', FREE_PARKING: 'PARKING' };
  function norm(r) {
    const g = r.game || {}, tiles = (r.board && r.board.tiles) || [];
    const players = (g.players || []).map((p) => ({ id: p.id, name: p.name, color: p.color, balance: p.money, position: p.position, bankrupt: p.is_bankrupt, properties: p.properties || [] }));
    const board = tiles.map((t) => ({ position: t.index, type: TT[t.type] || t.type, name: t.name, group: t.group, price: t.price, rent: t.rent || [], rent_base: (t.rent || [])[0] || 0, build_cost: t.house_cost, image_kind: t.image_kind, description: t.description, mortgage: t.mortgage_value, owner_id: t.owner_id, houses: t.houses >= 5 ? 0 : t.houses, hotel: t.houses >= 5, queen: t.is_queen }));
    const cur = g.current_turn_player_id, me = players.find((p) => p.id === meId);
    const mine = cur === meId && g.is_started && !g.is_over, rolled = !!g.dice_rolled_this_turn;
    const t = me && board[me.position];
    const buyable = t && t.price > 0 && !t.owner_id && ['PROPERTY', 'STATION', 'UTILITY'].includes(t.type);
    const win = players.find((p) => p.id === g.winner_id);
    return { city: r.city, board, players, current_turn: cur, status: g.is_over ? 'finished' : 'in_progress', winner: win && win.name, turn_start: g.turn_start_time, decision_deadline: g.decision_deadline || r.decision_deadline || null,
      can_roll: mine && !rolled, can_buy: mine && rolled && !!buyable, can_end_turn: mine && rolled,
      card: g.last_card_result && g.last_card_result.card ? { key: g.turn_index + ':' + g.last_card_result.card.id + ':' + g.last_card_result.player_id, deck: g.last_card_result.card.deck || g.last_card_result.card.type, title: g.last_card_result.card.title, delta: g.last_card_result.money_delta, jail: g.last_card_result.went_to_jail, who: g.last_card_result.player_id } : null };
  }

  // ---------- board ----------
  const STATION_ICONS = ['bolt', 'drop', 'wifi', 'train'];
  const CORNER = { START: 'Saída', JAIL: 'Prisão', PARKING: 'Visitante', GO_TO_JAIL: 'Vá para a Cadeia' };
  const side = (i, q) => (i < q ? 'L' : i < 2 * q ? 'T' : i < 3 * q ? 'R' : 'B');
  function cell(i, q) {
    const S = q + 1;
    if (i === 0) return [S, 1];
    if (i < q) return [S - i, 1];
    if (i === q) return [1, 1];
    if (i < 2 * q) return [1, i - q + 1];
    if (i === 2 * q) return [1, S];
    if (i < 3 * q) return [i - 2 * q + 1, S];
    if (i === 3 * q) return [S, S];
    return [S, S - (i - 3 * q)];
  }
  const use = (id) => `<svg aria-hidden="true"><use href="#i-${id}"/></svg>`;
  function tileIcon(t, n) {
    if (t.type === 'START') return 'arrow';
    if (t.type === 'JAIL') return 'bank';
    if (t.type === 'PARKING') return 'user';
    if (t.type === 'GO_TO_JAIL') return 'car';
    if (t.type === 'LUCK') return 'q';
    if (t.type === 'TAX') return 'coin';
    if (t.type === 'STATION' || t.type === 'UTILITY') return STATION_ICONS[n % 4];
    return 'house';
  }

  function buildBoard() {
    const board = $('board'), tiles = state.board, N = tiles.length, q = N / 4, S = q + 1;
    const tracks = `1.7fr repeat(${q - 1}, 1fr) 1.7fr`;
    board.style.gridTemplateColumns = tracks;
    board.style.gridTemplateRows = tracks;
    board.style.setProperty('--span', S - 2);
    board.innerHTML = '';
    let sn = 0;
    tiles.forEach((t, i) => {
      const [r, c] = cell(i, q), corner = i % q === 0;
      const el = document.createElement('button');
      el.type = 'button';
      el.className = 'tile ' + (corner ? 'corner ' : '') + 'tt-' + t.type.toLowerCase().replace(/_/g, '') + (t.type === 'PROPERTY' ? ' prop' : '') + (t.queen ? ' queen' : '');
      el.dataset.side = side(i, q);
      el.style.gridArea = `${r} / ${c}`;
      if (t.type === 'PROPERTY') el.style.setProperty('--gc', `var(--g-${t.group})`);
      const nm = CORNER[t.type] || (t.type === 'LUCK' ? 'Sorte ou Revés' : t.name);
      const icon = tileIcon(t, t.type === 'STATION' || t.type === 'UTILITY' ? sn++ : i);
      const th = ['PROPERTY', 'STATION', 'UTILITY'].includes(t.type) ? thumbFor(t) : null;
      el.innerHTML = `${th ? `<img class="th" src="${th}" alt="" onerror="this.remove()">` : ''}${use(icon)}<span class="nm">${esc(nm)}</span>${t.price && !corner ? `<span class="pr">${brl(t.price)}</span>` : ''}<span class="hs"></span>`;
      el.setAttribute('aria-label', `${nm}${t.price ? ', ' + brl(t.price) : ''}`);
      el.onclick = () => showTile(i);
      board.appendChild(el);
    });
    board.appendChild(centerEl());
    pawns.clear(); prevPos.clear(); boardBuilt = true;
  }

  function dieHtml(id) {
    const P = { 1: [5], 2: [1, 9], 3: [1, 5, 9], 4: [1, 3, 7, 9], 5: [1, 3, 5, 7, 9], 6: [1, 3, 4, 6, 7, 9] };
    const face = (v) => `<b class="f${v}">${P[v].map((p) => `<i style="grid-area:${Math.ceil(p / 3)}/${((p - 1) % 3) + 1}"></i>`).join('')}</b>`;
    return `<div class="dw"><div class="die" id="${id}">${[1, 2, 3, 4, 5, 6].map(face).join('')}</div></div>`;
  }
  function centerEl() {
    const c = document.createElement('div');
    c.className = 'center';
    c.innerHTML =
      `<div class="logo"><svg viewBox="0 0 64 32" aria-hidden="true"><path d="M4 30V16l8-6v20M16 30V8l9-6v28M29 30V12l9-4v22M42 30V4l10 6v20M56 30V14l4 2v14" fill="none" stroke="currentColor" stroke-width="2.500"/></svg>BANCO IMOBILIÁRIO</div>` +
      `<div class="piles"><div class="pile" title="Sorte">${use('q')}</div><div class="dice">${dieHtml('d1')}${dieHtml('d2')}</div><div class="pile" title="Revés">${use('house')}</div></div>`;
    return c;
  }

  // ---------- pawns ----------
  function placePawn(p, pos, idx = 0) {
    let el = pawns.get(p.id);
    if (!el) {
      el = document.createElement('div');
      el.className = 'pawn';
      el.innerHTML = '<i></i>';
      el.style.setProperty('--c', p.color);
      $('board').appendChild(el);
      pawns.set(p.id, el);
    }
    const t = $('board').children[pos];
    const off = [[-.18, -.18], [.18, -.18], [-.18, .18], [.18, .18]][idx % 4];
    el.style.transform = `translate(${t.offsetLeft + t.offsetWidth * (.5 + off[0])}px,${t.offsetTop + t.offsetHeight * (.5 + off[1])}px)`;
  }
  function placeAll() {
    if (!boardBuilt) return;
    state.players.forEach((p, i) => { if (!p.bankrupt) placePawn(p, p.position, i); prevPos.set(p.id, p.position); });
  }
  // hop tile to tile, squashing on landing (own `translate`/`scale` props, so they compose with the pawn's transform)
  function hop(p) {
    const el = pawns.get(p.id);
    if (!el || !maxMotion()) return;
    const h = $('board').offsetWidth * .04;
    el.firstChild.animate([{ translate: '0 0', scale: '1 1' }, { translate: `0 ${-h}px`, scale: '.92 1.1', offset: .45 }, { translate: '0 0', scale: '1.12 .86', offset: .85 }, { translate: '0 0', scale: '1 1' }], { duration: 200, easing: 'ease-out' });
  }
  function land(pos) {
    const t = $('board').children[pos];
    if (!t || !maxMotion()) return;
    t.classList.remove('land'); void t.offsetWidth; t.classList.add('land');
    setTimeout(() => t.classList.remove('land'), 1100);
  }
  async function walk(p, from, to) {
    const N = state.board.length, i = state.players.findIndex((x) => x.id === p.id);
    for (let pos = from; pos !== to;) { pos = (pos + 1) % N; placePawn(p, pos, i); hop(p); camera(pos); await sleep(200); }
    land(to);
  }

  // ---------- motion toggle (default: max, ignores the OS preference) + camera that follows the turn ----------
  let motion = 'max';
  try { if (localStorage.getItem('banco3d:motion') === 'reduced') motion = 'reduced'; } catch { /* storage blocked */ }
  const maxMotion = () => motion === 'max';
  function camera(pos) {
    const st = $('table').style, b = $('board');
    if (pos == null || !maxMotion() || $('table').classList.contains('flat') || !boardBuilt) {
      st.setProperty('--tx', '0px'); st.setProperty('--ty', '0px'); st.setProperty('--zm', '1'); return;
    }
    const t = b.children[pos], fx = (t.offsetLeft + t.offsetWidth / 2) / b.offsetWidth - .5, fy = (t.offsetTop + t.offsetHeight / 2) / b.offsetHeight - .5;
    st.setProperty('--tx', `${-fx * b.offsetWidth * .08}px`); st.setProperty('--ty', `${-fy * b.offsetHeight * .08}px`); st.setProperty('--zm', '1.04');
  }
  function followTurn() { const c = state && state.players.find((p) => p.id === state.current_turn); camera(c ? c.position : null); }
  function applyMotion() {
    document.documentElement.dataset.motion = motion;
    const b = $('btnMotion');
    b.textContent = 'Animações: ' + (maxMotion() ? 'Máximo' : 'Reduzido'); b.setAttribute('aria-pressed', !maxMotion());
    if (!maxMotion()) { camera(null); $('table').style.setProperty('--px', '0deg'); $('table').style.setProperty('--py', '0deg'); }
    else followTurn();
  }
  $('btnMotion').onclick = () => { motion = maxMotion() ? 'reduced' : 'max'; try { localStorage.setItem('banco3d:motion', motion); } catch { /* ignore */ } applyMotion(); };
  // light parallax: the tabletop leans a few degrees toward the pointer
  $('stage').addEventListener('pointermove', (e) => {
    if (!maxMotion() || e.pointerType === 'touch') return;
    const st = $('table').style;
    st.setProperty('--py', ((e.clientX / innerWidth - .5) * 5).toFixed(2) + 'deg'); st.setProperty('--px', ((.5 - e.clientY / innerHeight) * 3).toFixed(2) + 'deg');
  });
  applyMotion();

  // ---------- property art (assets/props): manifest or probing, graceful when absent ----------
  const KINDS = ['street', 'avenue', 'square', 'park', 'landmark', 'station', 'utility'];
  const KIND_LABEL = { street: 'Rua', avenue: 'Avenida', square: 'Praça', park: 'Parque', landmark: 'Ponto turístico', station: 'Estação', utility: 'Serviço' };
  const PROPS = {}; // kind -> [urls]
  const probe = (u) => new Promise((ok) => { const i = new Image(); i.onload = () => ok(true); i.onerror = () => ok(false); i.src = u; });
  const propsReady = (async () => {
    const base = 'assets/props/';
    let m = null;
    try { const r = await fetch(base + 'manifest.json'); if (r.ok) m = await r.json(); } catch { /* no manifest */ }
    const ext = (m && m.ext) || 'webp';
    await Promise.all(KINDS.map(async (k) => {
      const v = m ? m[k] : undefined, urls = [];
      if (Array.isArray(v)) v.forEach((f) => urls.push(base + f));
      else if (typeof v === 'number') for (let i = 1; i <= v; i++) urls.push(`${base}${k}-${i}.${ext}`);
      else if (!m) {
        for (let i = 1; i <= 8 && await probe(`${base}${k}-${i}.${ext}`); i++) urls.push(`${base}${k}-${i}.${ext}`);
        if (!urls.length && await probe(`${base}${k}.${ext}`)) urls.push(`${base}${k}.${ext}`);
      }
      PROPS[k] = urls;
    }));
  })();
  const hash = (str) => [...str].reduce((h, ch) => (h * 31 + ch.charCodeAt(0)) >>> 0, 7);
  function kindOf(t) {
    if (t.image_kind) return t.image_kind;
    if (t.queen) return 'landmark';
    if (t.type === 'STATION') return 'station';
    if (t.type === 'UTILITY') return 'utility';
    const n = t.name;
    if (/^av(\.|enida)?\s/i.test(n)) return 'avenue';
    if (/^(praça|pça|largo)\b/i.test(n)) return 'square';
    if (/(parque|jardim|bosque|lago)/i.test(n)) return 'park';
    if (/(museu|catedral|teatro|ópera|mercado|mirante|palácio|torre|estádio|centro histórico)/i.test(n)) return 'landmark';
    return 'street';
  }
  // same name -> same variant, so a tile keeps its picture across reloads
  function thumbFor(t) { const l = PROPS[kindOf(t)] || []; return l.length ? l[hash(t.name) % l.length] : null; }

  // ---------- players panel: persistent rows, balance tween, floating deltas ----------
  const rows = new Map(), bal = new Map();
  function tweenTo(id, el, to) {
    const b = bal.get(id), from = b.shown, t0 = performance.now();
    b.target = to; cancelAnimationFrame(b.raf);
    const step = (now) => {
      const k = Math.min(1, (now - t0) / 700);
      b.shown = Math.round(from + (to - from) * (1 - Math.pow(1 - k, 3)));
      el.textContent = brl(b.shown);
      if (k < 1) b.raf = requestAnimationFrame(step);
    };
    b.raf = requestAnimationFrame(step);
  }
  function renderPlayers() {
    const box = $('players');
    state.players.forEach((p) => {
      let r = rows.get(p.id);
      if (!r) {
        r = document.createElement('div');
        r.innerHTML = `<div class="av">${use('user')}</div><div class="who"><b></b><span class="bal"></span></div><span class="crown" aria-label="Vez de jogar">●</span>`;
        rows.set(p.id, r); box.appendChild(r);
        bal.set(p.id, { shown: p.balance, target: p.balance, raf: 0 });
        r.querySelector('.bal').textContent = brl(p.balance);
      }
      r.className = 'pl' + (p.id === state.current_turn ? ' turn' : '') + (p.bankrupt ? ' out' : '');
      r.style.setProperty('--c', p.color);
      r.querySelector('b').textContent = p.name + (p.id === meId ? ' (você)' : '');
      r.querySelector('.crown').hidden = p.id !== state.current_turn;
      const b = bal.get(p.id);
      if (b.target !== p.balance) {
        const d = p.balance - b.target, f = document.createElement('span');
        f.className = 'fly ' + (d > 0 ? 'up' : 'down'); f.textContent = (d > 0 ? '+' : '−') + Math.abs(d).toLocaleString('pt-BR');
        r.appendChild(f); setTimeout(() => f.remove(), 1600);
        tweenTo(p.id, r.querySelector('.bal'), p.balance);
      }
    });
  }

  // ---------- render ----------
  function render() {
    renderPlayers();
    pawns.forEach((el, id) => el.classList.toggle('turn', id === state.current_turn));
    followTurn();
    $('cityName').textContent = state.city || 'Sua cidade';
    const mine = state.current_turn === meId && state.status === 'in_progress';
    const cur = state.players.find((p) => p.id === state.current_turn);
    $('statusText').textContent = state.status === 'finished' ? `Fim! ${state.winner || ''} venceu` : mine ? 'Sua vez' : `Vez de ${cur ? cur.name : '…'}`;
    $('btnRoll').disabled = busy || !state.can_roll;
    $('btnBuy').disabled = busy || !state.can_buy;
    $('btnEnd').disabled = busy || !state.can_buy;
    $('btnBuild').disabled = true; $('btnBuild').title = 'Em breve';
    $('btnTrade').disabled = busy || state.status !== 'in_progress' || state.players.filter((p) => !p.bankrupt).length < 2;
    state.board.forEach((t, i) => {
      const el = $('board').children[i];
      const o = state.players.find((p) => p.id === t.owner_id);
      el.classList.toggle('owned', !!o);
      if (o) el.style.setProperty('--oc', o.color);
      el.querySelector('.hs').innerHTML = t.hotel ? '<i class="h"></i>' : '<i></i>'.repeat(t.houses || 0);
      el.querySelector('.nm').textContent = CORNER[t.type] || (t.type === 'LUCK' ? 'Sorte ou Revés' : t.name);
    });
  }

  let toastT;
  function toast(msg) {
    const t = $('toast');
    t.textContent = msg; t.hidden = false;
    clearTimeout(toastT); toastT = setTimeout(() => (t.hidden = true), 3600);
  }

  const FACE = { 1: [0, 0], 2: [0, -90], 3: [-90, 0], 4: [90, 0], 5: [0, 90], 6: [0, 180] };
  async function rollDice(dice) {
    ['d1', 'd2'].forEach((id, k) => {
      const d = $(id), [rx, ry] = FACE[dice[k]] || FACE[1], w = d.parentNode, n = (d._n = (d._n || 0) + 1);
      w.classList.remove('roll'); void w.offsetWidth; w.classList.add('roll');
      // spins on all 3 axes; always increasing so each roll tumbles forward; Z is a full turn so the face stays upright
      d.style.transform = `rotateX(${rx + 720 * n + k * 360}deg) rotateY(${ry + 720 * n}deg) rotateZ(${360 * (n + k)}deg)`;
    });
    await sleep(1200);
  }

  // apply a fresh room payload: roll dice + walk pawns for every mover, then redraw
  let seenCard; // undefined until the first state, so a reload doesn't replay an old card
  const cardMsg = (c) => { const p = state.players.find((x) => x.id === c.who); return `${c.deck === 'CHEST' ? 'Revés' : 'Sorte'}${p && p.id !== meId ? ' de ' + p.name : ''}: ${c.title}${c.delta ? ` (${c.delta > 0 ? '+' : '−'}${brl(Math.abs(c.delta))})` : ''}${c.jail ? ' · Cadeia!' : ''}`; };
  // events[]: only entries not seen before are new; the first payload just primes the set
  const seenEv = new Set();
  const evKey = (e) => e.seq ?? e.id ?? `${e.timestamp}|${e.type}|${e.player_id}`;
  function freshEvents(r) {
    const list = Array.isArray(r.events) ? r.events : (r.game && Array.isArray(r.game.events) ? r.game.events : null);
    if (!list) return null;
    const first = seenEv.size === 0 && !seenEv.primed;
    seenEv.primed = true;
    const out = list.filter((e) => !seenEv.has(evKey(e)));
    list.forEach((e) => seenEv.add(evKey(e)));
    if (seenEv.size > 500) seenEv.clear();
    return first ? [] : out;
  }
  // no events from the server: split the distance walked into two plausible dice
  function guessDice(from, to, N) {
    const d = (to - from + N) % N;
    if (d < 2 || d > 12) return null;
    const a = Math.ceil(d / 2), b = d - a;
    return Math.random() < .5 ? [a, b] : [b, a];
  }
  async function commit(r, quiet, rolled) {
    raw = r;
    const next = norm(r), old = new Map(prevPos);
    await propsReady;
    if (!boardBuilt || $('board').children.length < next.board.length) { state = next; buildBoard(); }
    state = next;
    const evs = freshEvents(r), diceBy = new Map(), N = state.board.length;
    (evs || []).filter((e) => e.type === 'roll' && e.dice).forEach((e) => diceBy.set(e.player_id, e.dice));
    for (const p of state.players) {
      const from = old.get(p.id);
      if (from !== undefined && from !== p.position && !p.bankrupt) {
        if (!(rolled && p.id === meId)) {
          const dice = diceBy.get(p.id) || guessDice(from, p.position, N);
          if (dice) { if (p.id !== meId) toast(`${p.name} tirou ${dice[0] + dice[1]}`); await rollDice(dice); }
        }
        if ((p.position - from + N) % N > 14) { placePawn(p, p.position, state.players.indexOf(p)); camera(p.position); land(p.position); } // card/jail teleport
        else await walk(p, from, p.position);
      } else if (!pawns.has(p.id)) placePawn(p, p.position, state.players.indexOf(p));
      prevPos.set(p.id, p.position);
    }
    render();
    checkOffer();
    autoTurn();
    if (evs) {
      const msgs = evs.filter((e) => e.message && e.type !== 'roll' && e.type !== 'pass').map((e) => e.message);
      if (msgs.length && !quiet) toast(msgs[msgs.length - 1]);
      return msgs.join(' · ');
    }
    const c = state.card;
    if (seenCard === undefined) { seenCard = c ? c.key : ''; return ''; }
    if (c && c.key !== seenCard) { seenCard = c.key; if (!quiet) toast(cardMsg(c)); return cardMsg(c); }
    return '';
  }

  // ---------- automatic turn: 8s window to buy, then the turn passes by itself ----------
  let autoKey = null, autoT = null, cdI = null;
  function stopAuto() { clearTimeout(autoT); clearInterval(cdI); autoT = cdI = autoKey = null; $('btnEnd').hidden = true; $('buyCd').hidden = true; }
  async function passTurn(msg) {
    if (busy) return void setTimeout(() => passTurn(msg), 400);
    await run(async () => {
      const fresh = await getRoom(roomId); // the server may have passed already
      if (fresh.game.current_turn_player_id !== meId) { await commit(fresh); return; }
      if (msg) toast(msg);
      const r = await act('pass');
      await commit(r.game ? r : await getRoom(roomId));
    });
  }
  function autoTurn() {
    const mine = state.current_turn === meId && state.status === 'in_progress';
    const key = `${state.current_turn}:${raw.game.turn_index}:${state.can_buy ? 'b' : state.can_end_turn ? 'p' : 'r'}`;
    if (key === autoKey) return;
    stopAuto(); autoKey = key;
    if (!mine) return;
    if (state.can_buy) {
      const server = !!state.decision_deadline, dl = server ? new Date(state.decision_deadline).getTime() : Date.now() + 8000;
      $('btnEnd').hidden = false; $('buyCd').hidden = false;
      const upd = () => { $('buyCd').textContent = Math.max(0, Math.ceil((dl - Date.now()) / 1000)); };
      cdI = setInterval(upd, 250); upd();
      autoT = setTimeout(() => passTurn('Tempo esgotado: não comprou'), Math.max(0, dl - Date.now()) + (server ? 2500 : 0)); // server passes first when it can
    } else if (state.can_end_turn) {
      autoT = setTimeout(() => passTurn(), 2200);
    }
  }

  // ---------- game actions ----------
  async function run(fn) {
    if (busy) return;
    busy = true; render();
    try { await fn(); } catch (e) { toast(e.message); }
    busy = false; if (state) render();
  }
  const act = (name, extra) => api('/api/banco3d/room/' + name, { room_id: roomId, player_id: meId, ...extra });

  $('btnRoll').onclick = () => run(async () => {
    const r = await act('roll'); // the server rolls the dice
    const d = r.dice || r.result || {};
    const dice = [d.die1 || (d.dice && d.dice[0]) || 1, d.die2 || (d.dice && d.dice[1]) || 1];
    await rollDice(dice);
    const card = await commit(r.game ? r : await getRoom(roomId), true, true);
    const me = state.players.find((p) => p.id === meId), t = state.board[me.position];
    toast(`Tirou ${dice[0] + dice[1]}. ${t.name}.` + (state.can_buy ? ` Comprar por ${brl(t.price)}?` : '') + (card ? ' ' + card : ''));
  });
  $('btnBuy').onclick = () => run(async () => { const r = await act('buy'); await commit(r.game ? r : await getRoom(roomId)); toast('Comprado!'); });
  $('btnEnd').onclick = () => passTurn();
  $('btnMap').onclick = () => { const f = $('table').classList.toggle('flat'); $('btnMap').setAttribute('aria-pressed', f); if (f) camera(null); else followTurn(); setTimeout(placeAll, 1050); };

  let cardTimer = null;
  function sheet(title, html) { clearInterval(cardTimer); $('sheetTitle').textContent = title; $('sheetBody').innerHTML = html; $('sheet').hidden = false; }
  $('sheetClose').onclick = () => { clearInterval(cardTimer); $('sheet').hidden = true; };
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') $('sheet').hidden = true; });
  function showTile(i) {
    const t = state.board[i], o = state.players.find((p) => p.id === t.owner_id);
    if (!t.price) return sheet(CORNER[t.type] || (t.type === 'LUCK' ? 'Sorte ou Revés' : t.name), t.description ? `<p>${esc(t.description)}</p>` : '<p class="dim">Casa especial.</p>');
    const k = kindOf(t), list = PROPS[k] || [], idx = list.length ? hash(t.name) % list.length : -1, gc = t.type === 'PROPERTY' ? `var(--g-${t.group})` : '#38bdf8';
    const rent = t.type === 'PROPERTY' && t.rent.length > 5
      ? ['Aluguel base', '1 casa', '2 casas', '3 casas', '4 casas', 'Hotel'].map((l, n) => `<dt>${l}</dt><dd>${brl(t.rent[n])}</dd>`).join('')
      : (t.rent_base ? `<dt>Aluguel</dt><dd>${brl(t.rent_base)}</dd>` : '');
    const layers = idx < 0 ? '' : `<img class="on" src="${list[idx]}" alt="">${list.length > 1 ? '<img alt="">' : ''}`;
    sheet(t.name, `<div class="pc"><div class="pc-img" style="--gc:${gc}">${layers}<span class="pc-ic">${use(tileIcon(t, i))}</span><span class="pc-cap">${KIND_LABEL[k] || ''}${t.queen ? ' · ★ ícone da cidade' : ''}</span></div>${t.description ? `<p>${esc(t.description)}</p>` : ''}<dl><dt>Preço</dt><dd>${brl(t.price)}</dd>${rent}${t.build_cost ? `<dt>Casa custa</dt><dd>${brl(t.build_cost)}</dd>` : ''}${t.mortgage ? `<dt>Hipoteca</dt><dd>${brl(t.mortgage)}</dd>` : ''}<dt>Dono</dt><dd>${o ? esc(o.name) : 'Livre'}</dd></dl></div>`);
    // every ~6s fade to the next variant of this kind (skipped with reduced motion)
    const imgs = $('sheetBody').querySelectorAll('.pc-img img');
    if (imgs.length === 2 && maxMotion()) {
      let cur = idx, front = 0;
      cardTimer = setInterval(() => {
        cur = (cur + 1) % list.length;
        const back = imgs[1 - front];
        back.onload = () => { imgs[front].classList.remove('on'); back.classList.add('on'); front = 1 - front; };
        back.src = list[cur];
      }, 6000);
    }
  }

  // ---------- incoming trade offer ----------
  let offerKey = null;
  const tilesTxt = (ids, cash) => [...ids.map((i) => state.board[i].name), cash ? brl(cash) : ''].filter(Boolean).join(', ') || 'nada';
  function checkOffer() {
    const t = raw && raw.pending_trade;
    if (!t || t.status !== 'pending') { if (offerKey) { offerKey = null; if ($('sheetTitle').textContent === 'Proposta de troca') $('sheet').hidden = true; } return; }
    if (t.receiver_id !== meId) return;
    const key = t.id + t.created_at;
    if (key === offerKey) return;
    offerKey = key;
    const from = state.players.find((p) => p.id === t.sender_id);
    sheet('Proposta de troca', `<p><b>${esc(from ? from.name : 'Jogador')}</b> oferece:</p><p>${esc(tilesTxt(t.offered_tiles || [], t.offered_cash))}</p><p>e pede:</p><p>${esc(tilesTxt(t.requested_tiles || [], t.requested_cash))}</p><div class="row"><button class="mini ghost" id="offNo" type="button">Recusar</button><button class="mini" id="offYes" type="button">Aceitar</button></div>`);
    const answer = (action) => run(async () => {
      try { const r = await api('/api/banco3d/room/trade', { room_id: roomId, action, player_id: meId }); await commit(r.game ? r : await getRoom(roomId)); toast(action === 'accept' ? 'Troca feita!' : 'Proposta recusada'); } finally { $('sheet').hidden = true; }
    });
    $('offYes').onclick = () => answer('accept');
    $('offNo').onclick = () => answer('decline');
  }

  // ---------- trade modal ----------
  let tradeOpener = null;
  const mineOf = (id) => state.board.filter((t) => t.owner_id === id && t.houses === 0 && !t.hotel);
  function tileChecks(list, name) {
    return list.length ? list.map((t) => `<label class="chk"><input type="checkbox" name="${name}" value="${t.position}"><span>${esc(t.name)}</span></label>`).join('') : '<p class="none">Sem imóveis livres de construção.</p>';
  }
  function fillTrade() {
    const f = $('tradeForm'), to = f.to.value;
    $('tradeGive').innerHTML = tileChecks(mineOf(meId), 'give');
    $('tradeWant').innerHTML = tileChecks(mineOf(to), 'want');
  }
  function openTrade() {
    const others = state.players.filter((p) => p.id !== meId && !p.bankrupt);
    $('tradeTo').innerHTML = others.map((p) => `<option value="${esc(p.id)}">${esc(p.name)}</option>`).join('');
    $('tradeForm').giveCash.value = 0; $('tradeForm').wantCash.value = 0;
    $('tradeMsg').textContent = 'A troca vale na hora, sem aceite do outro jogador.'; $('tradeMsg').classList.remove('bad');
    tradeOpener = document.activeElement; fillTrade(); $('trade').hidden = false; $('tradeTo').focus();
  }
  function closeTrade() { $('trade').hidden = true; if (tradeOpener && tradeOpener.focus) tradeOpener.focus(); }
  $('btnTrade').onclick = openTrade;
  $('tradeTo').onchange = fillTrade;
  $('tradeX').onclick = $('tradeCancel').onclick = closeTrade;
  $('trade').addEventListener('keydown', (e) => { if (e.key === 'Escape') closeTrade(); });
  $('tradeForm').onsubmit = (e) => {
    e.preventDefault();
    const f = e.target, ids = (n) => [...f.querySelectorAll(`[name=${n}]:checked`)].map((i) => +i.value);
    const body = { room_id: roomId, action: 'propose', player_id: meId, to_player_id: f.to.value, give: { props: ids('give'), cash: +f.giveCash.value || 0 }, want: { props: ids('want'), cash: +f.wantCash.value || 0 } };
    if (!body.give.props.length && !body.want.props.length && !body.give.cash && !body.want.cash) { $('tradeMsg').textContent = 'Escolha algo para trocar.'; $('tradeMsg').classList.add('bad'); return; }
    run(async () => {
      try { const r = await api('/api/banco3d/room/trade', body); await commit(r.game ? r : await getRoom(roomId)); closeTrade(); toast(r.pending_trade ? 'Proposta enviada. Aguardando resposta.' : 'Troca feita!'); }
      catch (x) { $('tradeMsg').textContent = x.message; $('tradeMsg').classList.add('bad'); }
    });
  };

  // ---------- match config modal ----------
  const CFG_KEY = 'banco3d:config';
  const DEFAULT_CFG = { city: '', players: 4, balance: 1500, bots: 0, difficulty: 'normal', turn: 30, bail: true, triple: true, freecard: true };
  let cfg = { ...DEFAULT_CFG };
  try { cfg = { ...cfg, ...JSON.parse(localStorage.getItem(CFG_KEY)) }; } catch { /* first run */ }
  let cfgOpener = null;
  const cf = () => $('cfgForm');
  function fillCfg() {
    const f = cf();
    f.city.value = $('fHome').city.value;
    f.balance.value = cfg.balance;
    for (const k of ['players', 'bots', 'difficulty', 'turn']) f.querySelectorAll(`[name=${k}]`).forEach((r) => (r.checked = String(r.value) === String(cfg[k])));
    for (const k of ['bail', 'triple', 'freecard']) f[k].checked = cfg[k];
    syncBots();
  }
  // bots never exceed free seats; difficulty is moot without bots
  function syncBots() {
    const f = cf(), max = +f.players.value - 1;
    f.querySelectorAll('[name=bots]').forEach((r) => { r.disabled = +r.value > max; });
    if (+f.bots.value > max) f.querySelector(`[name=bots][value="${max}"]`).checked = true;
    $('diffSegs').classList.toggle('off', +f.bots.value === 0);
  }
  function openCfg() { cfgOpener = document.activeElement; fillCfg(); $('cfg').hidden = false; cf().city.focus(); }
  function closeCfg() { $('cfg').hidden = true; if (cfgOpener && cfgOpener.focus) cfgOpener.focus(); }
  cf().addEventListener('change', syncBots);
  $('bCfg').onclick = openCfg;
  $('cfgX').onclick = $('cfgCancel').onclick = closeCfg;
  $('cfg').addEventListener('keydown', (e) => { if (e.key === 'Escape') closeCfg(); });
  cf().onsubmit = (e) => {
    e.preventDefault();
    const f = cf();
    cfg = { players: +f.players.value, balance: Math.min(10000, Math.max(500, +f.balance.value || 1500)), bots: +f.bots.value, difficulty: f.difficulty.value, turn: +f.turn.value, bail: f.bail.checked, triple: f.triple.checked, freecard: f.freecard.checked };
    try { localStorage.setItem(CFG_KEY, JSON.stringify(cfg)); } catch { /* private mode */ }
    $('fHome').city.value = f.city.value.trim();
    closeCfg();
  };
  const cfgPayload = () => ({ max_players: cfg.players, bots: { count: cfg.bots, difficulty: cfg.difficulty }, starting_balance: cfg.balance, turn_seconds: cfg.turn, rules: { jail_bail: cfg.bail, jail_triple_doubles: cfg.triple, get_out_free_card: cfg.freecard } });

  // ---------- lobby / voting / room flow ----------
  const setView = (v) => {
    $('start').dataset.view = v; $('start').hidden = v === 'game';
    if (v === 'game') { const t = $('table'); t.classList.remove('enter'); void t.offsetWidth; t.classList.add('enter'); } // flat -> tilt entrance
  };
  const err = (m) => ($('startErr').textContent = m || '');
  const newCode = () => Array.from({ length: 5 }, () => 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'[Math.floor(Math.random() * 32)]).join('');
  const parseCode = (v) => { v = v.trim(); try { return new URL(v).searchParams.get('sala') || v; } catch { return v.toUpperCase(); } };
  const isHost = () => raw && raw.game.players[0] && raw.game.players[0].id === meId;
  const roomLink = () => { const u = new URL(location.href); u.search = ''; u.searchParams.set('sala', roomId); if (API) u.searchParams.set('api', API); if (DEMO) u.searchParams.set('demo', ''); return u.toString(); };

  async function join(id, name) {
    const st = await getRoom(id).catch(() => { throw new Error('Sala não encontrada'); });
    const saved = loadSaved(id);
    const j = await api('/api/banco3d/room/join', { room_id: id, player_id: saved ? saved.player_id : '', name, color: COLORS[st.game.players.length % COLORS.length], piece: 'car' });
    roomId = id; meId = j.player.id;
    save(id, { player_id: meId, name });
    try { localStorage.setItem('banco3d:last', id); } catch { /* private mode */ }
    const u = new URL(location.href); u.searchParams.set('sala', id); history.replaceState(null, '', u);
    startPolling();
  }

  $('fHome').onsubmit = async (e) => {
    e.preventDefault(); err();
    const f = e.target;
    try {
      const id = newCode();
      const c = await api('/api/banco3d/room/create', { room_id: id, city: f.city.value.trim(), place_id: f.city.dataset.placeId || undefined, config: cfgPayload() });
      localVoting = { candidates: c.candidates };
      await join(id, f.name.value.trim());
    } catch (x) { err(x.message); }
  };
  $('bCode').onclick = async () => {
    err();
    const f = $('fHome'), id = parseCode(f.code.value);
    if (!id) return err('Digite o código da sala.');
    try { await join(id, f.name.value.trim() || 'Jogador'); } catch (x) { err(x.message); }
  };
  $('fJoin').onsubmit = async (e) => {
    e.preventDefault(); err();
    try { await join(roomId, e.target.name.value.trim()); } catch (x) { err(x.message); }
  };
  $('bCopy').onclick = async () => { try { await navigator.clipboard.writeText(roomLink()); toast('Link copiado'); } catch { $('lbLink').select(); } };
  $('bVote').onclick = async () => {
    try { const r = await api('/api/banco3d/room/vote', { room_id: roomId, player_id: meId, action: 'start' }); localVoting = { candidates: r.candidates, deadline: r.deadline }; } catch (x) { err(x.message); }
    tick();
  };

  const candidates = () => (raw.voting && raw.voting.candidates) || raw.candidates || (localVoting && localVoting.candidates) || [];
  function votingInfo() {
    const v = raw.voting || localVoting;
    return v && v.deadline ? v : null;
  }
  const secsLeft = (v) => Math.max(0, Math.min(VOTE_SECS, Math.ceil((new Date(v.deadline).getTime() - Date.now()) / 1000)));

  function drawLobby() {
    $('lbCity').textContent = raw.city;
    $('lbCands').innerHTML = candidates().map((c) => `<li>${esc(c)}</li>`).join('');
    $('lbPlayers').innerHTML = raw.game.players.map((p) => `<li style="--c:${esc(p.color)}"><i></i>${esc(p.name)}${p.id === meId ? ' (você)' : ''}</li>`).join('');
    $('lbLink').value = roomLink();
    $('bVote').hidden = !isHost(); $('lbWait').hidden = isHost();
  }

  async function drawVote() {
    const v = votingInfo(), left = secsLeft(v), c = candidates(), counts = c.map(() => 0);
    Object.values((raw.voting && raw.voting.votes) || {}).forEach((i) => { if (counts[i] !== undefined) counts[i]++; });
    const winner = (raw.voting && raw.voting.winner) || (localVoting && localVoting.winner);
    $('vCount').textContent = left;
    $('vBar').style.transform = `scaleX(${left / VOTE_SECS})`;
    $('vOpts').innerHTML = c.map((n, i) => `<button type="button" data-i="${i}" class="${i === myVote ? 'sel' : ''}${winner === n ? ' win' : ''}" ${left === 0 ? 'disabled' : ''}>${esc(n)}<b>${counts[i] || ''}</b></button>`).join('');
    $('vOpts').querySelectorAll('button').forEach((b) => (b.onclick = async () => {
      myVote = +b.dataset.i;
      try { await api('/api/banco3d/room/vote', { room_id: roomId, player_id: meId, candidate: myVote, action: 'vote' }); } catch (x) { err(x.message); }
      drawVote();
    }));
    $('vMsg').textContent = left === 0 ? (winner ? `Vencedor: ${winner}` : 'Apurando…') : 'Toque para votar. Dá para mudar o voto.';
    if (left === 0 && !closing && !winner) {
      closing = true;
      try { const r = await api('/api/banco3d/room/vote', { room_id: roomId, player_id: meId, action: 'close' }); localVoting = { ...(localVoting || {}), winner: r.winner }; } catch (x) { err(x.message); }
      closing = false;
    }
    if (left === 0 && winner && isHost() && !starting) {
      starting = true;
      try { await act('start'); } catch (x) { err(x.message); starting = false; }
    }
  }

  async function tick() {
    if (!roomId || polling || busy) return;
    polling = true;
    try {
      const r = await getRoom(roomId);
      raw = r;
      if (r.game.is_started) {
        if (!$('start').hidden) { setView('game'); boardBuilt = false; }
        await commit(r);
        if (lastTurn !== null && lastTurn !== state.current_turn && state.status === 'in_progress') {
          const cur = state.players.find((p) => p.id === state.current_turn);
          toast(state.current_turn === meId ? 'Sua vez!' : `Vez de ${cur ? cur.name : '…'}`);
        }
        lastTurn = state.current_turn;
      } else if (votingInfo()) {
        setView('vote'); await drawVote();
      } else { setView('lobby'); drawLobby(); }
    } catch (x) {
      if (/não encontrada|not found/i.test(x.message)) { clearInterval(timer); forget(roomId); roomId = null; stopAuto(); setView('home'); err('A sala não existe mais (o servidor pode ter reiniciado). Crie uma nova.'); }
    }
    polling = false;
  }
  let timer;
  function startPolling() { clearInterval(timer); tick(); timer = setInterval(tick, 1000); }
  addEventListener('resize', () => state && placeAll());

  // ---------- city combobox: GET /api/banco3d/places/suggest?q= (free text always allowed) ----------
  function attachCombo(input) {
    const wrap = document.createElement('div');
    wrap.className = 'combo';
    input.replaceWith(wrap); wrap.appendChild(input);
    const lb = document.createElement('ul');
    lb.className = 'combo-list'; lb.id = 'lb-' + Math.random().toString(36).slice(2, 7); lb.hidden = true; lb.setAttribute('role', 'listbox'); lb.setAttribute('aria-label', 'Sugestões de lugares');
    wrap.appendChild(lb);
    input.setAttribute('role', 'combobox'); input.setAttribute('aria-autocomplete', 'list'); input.setAttribute('aria-expanded', 'false'); input.setAttribute('aria-controls', lb.id); input.autocomplete = 'off';
    let items = [], act = -1, timer, ctl;
    const open = (v) => { lb.hidden = !v; input.setAttribute('aria-expanded', String(v)); wrap.toggleAttribute('data-open', v); if (!v) { act = -1; input.removeAttribute('aria-activedescendant'); } };
    const note = (t) => { items = []; lb.innerHTML = `<li class="note" role="presentation">${t}</li>`; open(true); };
    const draw = () => {
      lb.innerHTML = items.map((it, i) => `<li role="option" id="${lb.id}-${i}" aria-selected="${i === act}" class="${i === act ? 'on' : ''}"><b>${esc(it.name)}</b>${it.desc ? `<span>${esc(it.desc)}</span>` : ''}</li>`).join('');
      if (act >= 0) input.setAttribute('aria-activedescendant', `${lb.id}-${act}`); else input.removeAttribute('aria-activedescendant');
    };
    const pick = (i) => { const it = items[i]; input.value = it.desc ? `${it.name}, ${it.desc}` : it.name; input.dataset.placeId = it.id; open(false); };
    lb.addEventListener('mousedown', (e) => { const li = e.target.closest('[role=option]'); if (li) { e.preventDefault(); pick(+li.id.split('-').pop()); } });
    input.addEventListener('input', () => {
      delete input.dataset.placeId; clearTimeout(timer); if (ctl) ctl.abort();
      const q = input.value.trim();
      if (q.length < 2) return open(false);
      timer = setTimeout(async () => {
        note('Buscando…'); ctl = new AbortController();
        try {
          const d = await api('/api/banco3d/places/suggest?q=' + encodeURIComponent(q), null, ctl.signal);
          const list = Array.isArray(d) ? d : d.suggestions || d.places || d.results || [];
          items = list.slice(0, 6).map((it) => ({ id: it.id || it.place_id || '', name: it.name || it.label || it.description || '', desc: it.name || it.label ? it.description || it.subtitle || '' : '' })).filter((it) => it.name);
          if (!items.length) return note('Nenhum lugar encontrado. Pode digitar livremente.');
          act = -1; draw(); open(true);
        } catch (x) { if (x.name !== 'AbortError') note('Sugestões indisponíveis. Digite livremente.'); }
      }, 300);
    });
    input.addEventListener('keydown', (e) => {
      if (lb.hidden || !items.length) { if (e.key === 'Escape' && !lb.hidden) { e.stopPropagation(); open(false); } return; }
      if (e.key === 'ArrowDown') { e.preventDefault(); act = (act + 1) % items.length; draw(); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); act = (act - 1 + items.length) % items.length; draw(); }
      else if (e.key === 'Enter' && act >= 0) { e.preventDefault(); pick(act); }
      else if (e.key === 'Escape') { e.stopPropagation(); open(false); }
    });
    input.addEventListener('blur', () => setTimeout(() => open(false), 120));
  }
  attachCombo($('fHome').city);
  attachCombo(cf().city);

  // ---------- boot: reopen the room from ?sala= (or the last one) via saved player_id ----------
  (async () => {
    if (qs.get('config') === '1') openCfg();
    let id = (qs.get('sala') || '').toUpperCase(), auto = false;
    if (!id) { try { id = localStorage.getItem('banco3d:last') || ''; auto = !!id; } catch { /* private mode */ } }
    if (!id) return setView('home');
    $('joinCode').textContent = id; roomId = id;
    for (let n = 0; n < 3; n++) {
      try {
        const st = await getRoom(id);
        $('joinCity').textContent = st.city;
        const saved = loadSaved(id);
        if (saved) await join(id, saved.name || 'Jogador');
        else if (auto) { roomId = null; setView('home'); } else setView('join');
        return;
      } catch (x) {
        if (/não encontrada|not found|404/i.test(x.message)) { roomId = null; forget(id); setView('home'); if (!auto) err('Essa sala não existe mais (o servidor pode ter reiniciado). Crie uma nova.'); return; }
        await sleep(1500);
      }
    }
    roomId = null; setView('home'); err('Sem conexão com o servidor. Recarregue a página.');
  })();

  // ---------- demo engine (?demo): in-memory banco3d backend for visual checks ----------
  const R = {};
  function demoApi(path, b) {
    const ok = (v) => Promise.resolve(structuredClone(v)), no = (m) => Promise.reject(new Error(m));
    const P = path.split('?')[0];
    if (P.endsWith('/places/suggest')) {
      const q = decodeURIComponent(path.split('q=')[1] || '').toLowerCase();
      return ok({ suggestions: [['Curitiba', 'Paraná, Brasil'], ['Cuiabá', 'Mato Grosso, Brasil'], ['Cusco', 'Peru'], ['Recife', 'Pernambuco, Brasil'], ['Rio de Janeiro', 'Brasil'], ['Roma', 'Itália']].filter(([n]) => n.toLowerCase().includes(q)).map(([name, description], i) => ({ id: 'pl' + i, name, description })) });
    }
    if (P.endsWith('/room/create')) {
      const names = ['Rua das Flores', 'Rua do Sol', 'Rua Aurora', 'Rua dos Ipês', 'Estação de Energia', 'Avenida Central', 'Avenida Paulista', 'Avenida Brasil', 'Avenida Atlântica', 'Rua 6 de Novembro', 'Rua Sete de Setembro', 'Rua Marechal Deodoro', 'Rua Floriano Peixoto', 'Estação de Água', 'Praça da República', 'Praça XV', 'Praça Mauá', 'Praça Tiradentes', 'Rua da Liberdade', 'Rua Galvão Bueno', 'Rua 25 de Março', 'Rua Direita', 'Estação de Trem', 'Rua Augusta', 'Rua Oscar Freire', 'Rua Haddock Lobo', 'Rua da Consolação', 'Rua Carlos Gomes', 'Rua Uruguai', 'Rua Rio Branco', 'Rua da Bahia', 'Estação de Internet', 'Rua Prudente de Morais'];
      const G = ['brown', 'light_blue', 'pink', 'orange', 'red', 'yellow', 'green', 'dark_blue'];
      const tiles = Array.from({ length: 40 }, (_, i) => {
        const t = { index: i, name: '', type: 'PROPERTY', group: G[Math.min(7, Math.floor(i / 5))], price: 60 + i * 9, rent: [2 + i], house_cost: 50, owner_id: '', houses: 0, is_queen: i === 39 };
        const sp = { 0: ['GO', 'Saída'], 10: ['JAIL', 'Prisão'], 20: ['FREE_PARKING', 'Visitante'], 30: ['GO_TO_JAIL', 'Vá para a Cadeia'], 4: ['TAX', 'Imposto'], 38: ['TAX', 'Imposto'], 7: ['CHANCE', 'Sorte'], 22: ['CHANCE', 'Sorte'], 17: ['CHEST', 'Revés'], 33: ['CHEST', 'Revés'] };
        if (sp[i]) Object.assign(t, { type: sp[i][0], name: sp[i][1], price: 0, group: 'none' });
        else if (i % 5 === 0) Object.assign(t, { type: 'STATION', name: names.shift(), price: 200, group: 'none' });
        else t.name = names.shift() || 'Rua ' + i;
        return t;
      });
      R[b.room_id] = { id: b.room_id, city: b.city, board: { city: b.city, tiles }, candidates: ['Avenida Central', 'Jardim Botânico', 'Rua XV de Novembro', 'Parque Barigui'], voting: null,
        game: { players: [], current_turn_player_id: '', dice_rolled_this_turn: false, is_started: false, is_over: false } };
      return ok({ room_id: b.room_id, city: b.city, candidates: R[b.room_id].candidates });
    }
    const rid = b ? b.room_id : new URLSearchParams(path.split('?')[1]).get('room_id'), room = R[rid];
    if (!room) return no('room not found');
    const g = room.game;
    if (P.endsWith('/room/state')) { g.players.slice(1).forEach(() => {}); return ok(room); }
    if (P.endsWith('/room/join')) {
      let p = g.players.find((x) => x.id === b.player_id);
      if (!p) { p = { id: 'p' + (g.players.length + 1), name: b.name, color: b.color, piece: b.piece, money: 1500, position: 0, is_bankrupt: false, properties: [] }; g.players.push(p);
        if (g.players.length === 1) ['Gabriel', 'Larissa'].forEach((n, k) => g.players.push({ id: 'b' + k, name: n, color: COLORS[k + 1], money: 1500, position: 0, properties: [] })); }
      return ok({ reconnected: false, player: p });
    }
    if (P.endsWith('/room/vote')) {
      if (b.action === 'start') { room.voting = { candidates: room.candidates, deadline: new Date(Date.now() + VOTE_SECS * 1000).toISOString(), votes: {}, is_closed: false }; return ok({ status: 'voting_started', candidates: room.candidates, deadline: room.voting.deadline }); }
      if (b.action === 'close') { const c = [0, 0, 0, 0]; Object.values(room.voting.votes).forEach((i) => c[i]++); const w = room.candidates[c.indexOf(Math.max(...c))]; room.voting.winner = w; room.voting.is_closed = true; room.board.tiles[39].name = w; return ok({ winner: w }); }
      room.voting.votes[b.player_id] = b.candidate; return ok({ status: 'vote_recorded' });
    }
    if (P.endsWith('/room/start')) { g.is_started = true; g.current_turn_player_id = g.players[0].id; return ok(room); }
    const me = g.players.find((x) => x.id === (b.player_id || b.sender_id));
    if (P.endsWith('/room/roll')) {
      const d1 = 1 + Math.floor(Math.random() * 6), d2 = 1 + Math.floor(Math.random() * 6);
      me.position = (me.position + d1 + d2) % 40; g.dice_rolled_this_turn = true;
      const tt = room.board.tiles[me.position].type;
      if (tt === 'CHANCE' || tt === 'CHEST') { const dl = [200, -100, 50][Math.floor(Math.random() * 3)]; me.money += dl; g.turn_index = (g.turn_index || 0) + 1; g.last_card_result = { card: { id: 'c' + g.turn_index, deck: tt, title: tt === 'CHANCE' ? 'Feira de artesanato lotada' : 'Conserto do bonde', text: '' }, player_id: me.id, money_delta: dl }; }
      return ok({ ...room, dice: { die1: d1, die2: d2 } });
    }
    if (P.endsWith('/room/buy')) { const t = room.board.tiles[me.position]; t.owner_id = me.id; me.money -= t.price; me.properties.push(t.index); return ok(room); }
    if (P.endsWith('/room/trade')) {
      const swap = (t) => { const a = g.players.find((x) => x.id === t.sender_id), o = g.players.find((x) => x.id === t.receiver_id); t.offered_tiles.forEach((k) => { room.board.tiles[k].owner_id = o.id; }); t.requested_tiles.forEach((k) => { room.board.tiles[k].owner_id = a.id; }); a.money += t.requested_cash - t.offered_cash; o.money += t.offered_cash - t.requested_cash; };
      if (b.action === 'accept') { swap(room.pending_trade); room.pending_trade = null; return ok(room); }
      if (b.action === 'decline') { room.pending_trade = null; return ok(room); }
      const t = { id: 'trade-1', created_at: String(Date.now()), status: 'pending', sender_id: b.player_id, receiver_id: b.to_player_id, offered_tiles: b.give.props, offered_cash: b.give.cash, requested_tiles: b.want.props, requested_cash: b.want.cash };
      if (b.to_player_id.startsWith('b')) swap(t); else room.pending_trade = t;
      return ok(room);
    }
    if (P.endsWith('/room/pass')) {
      room.events = room.events || [];
      g.players.slice(1).forEach((p) => {
        const d1 = 1 + Math.floor(Math.random() * 6), d2 = 1 + Math.floor(Math.random() * 6);
        p.position = (p.position + d1 + d2) % 40;
        room.events.push({ type: 'roll', player_id: p.id, player_name: p.name, dice: [d1, d2], timestamp: Date.now() + '' + p.id });
        if (Math.random() < .5) { const a = 20 * (1 + Math.floor(Math.random() * 4)); p.money -= a; g.players[0].money += a; room.events.push({ type: 'rent', player_id: p.id, player_name: p.name, value: a, message: `${p.name} pagou $ ${a} de aluguel`, timestamp: Date.now() + 'r' + p.id }); }
      });
      room.events = room.events.slice(-30); g.dice_rolled_this_turn = false; g.turn_index = (g.turn_index || 0) + 1;
      return ok(room);
    }
    return no('Indisponível no modo demo');
  }
})();
