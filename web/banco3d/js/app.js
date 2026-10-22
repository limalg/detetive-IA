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
  async function api(path, body) {
    if (DEMO) return demoApi(path, body);
    const res = await fetch(API + path, {
      method: body ? 'POST' : 'GET',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
      credentials: 'include',
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || data.message || 'Erro ' + res.status);
    return data;
  }
  const getRoom = (id) => api('/api/banco3d/room/state?room_id=' + encodeURIComponent(id));

  // saved player id per room, so a reload reconnects instead of re-joining
  const saveKey = (id) => 'banco3d:' + id;
  const loadSaved = (id) => { try { return JSON.parse(localStorage.getItem(saveKey(id))); } catch { return null; } };
  const save = (id, v) => { try { localStorage.setItem(saveKey(id), JSON.stringify(v)); } catch { /* private mode */ } };

  // ---------- backend shape -> view shape ----------
  const TT = { GO: 'START', CHANCE: 'LUCK', CHEST: 'LUCK', FREE_PARKING: 'PARKING' };
  function norm(r) {
    const g = r.game || {}, tiles = (r.board && r.board.tiles) || [];
    const players = (g.players || []).map((p) => ({ id: p.id, name: p.name, color: p.color, balance: p.money, position: p.position, bankrupt: p.is_bankrupt, properties: p.properties || [] }));
    const board = tiles.map((t) => ({ position: t.index, type: TT[t.type] || t.type, name: t.name, group: t.group, price: t.price, rent_base: (t.rent || [])[0] || 0, build_cost: t.house_cost, owner_id: t.owner_id, houses: t.houses >= 5 ? 0 : t.houses, hotel: t.houses >= 5, queen: t.is_queen }));
    const cur = g.current_turn_player_id, me = players.find((p) => p.id === meId);
    const mine = cur === meId && g.is_started && !g.is_over, rolled = !!g.dice_rolled_this_turn;
    const t = me && board[me.position];
    const buyable = t && t.price > 0 && !t.owner_id && ['PROPERTY', 'STATION', 'UTILITY'].includes(t.type);
    const win = players.find((p) => p.id === g.winner_id);
    return { city: r.city, board, players, current_turn: cur, status: g.is_over ? 'finished' : 'in_progress', winner: win && win.name, turn_start: g.turn_start_time,
      can_roll: mine && !rolled, can_buy: mine && rolled && !!buyable, can_end_turn: mine && rolled };
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
      el.className = 'tile ' + (corner ? 'corner ' : '') + t.type.toLowerCase().replace(/_/g, '') + (t.type === 'PROPERTY' ? ' prop' : '') + (t.queen ? ' queen' : '');
      el.dataset.side = side(i, q);
      el.style.gridArea = `${r} / ${c}`;
      if (t.type === 'PROPERTY') el.style.setProperty('--gc', `var(--g-${t.group})`);
      const nm = CORNER[t.type] || (t.type === 'LUCK' ? 'Sorte ou Revés' : t.name);
      const icon = tileIcon(t, t.type === 'STATION' || t.type === 'UTILITY' ? sn++ : i);
      el.innerHTML = `${use(icon)}<span class="nm">${esc(nm)}</span>${t.price && !corner ? `<span class="pr">${brl(t.price)}</span>` : ''}<span class="hs"></span>`;
      el.setAttribute('aria-label', `${nm}${t.price ? ', ' + brl(t.price) : ''}`);
      el.onclick = () => showTile(i);
      board.appendChild(el);
    });
    board.appendChild(centerEl());
    pawns.clear(); prevPos.clear(); boardBuilt = true;
  }

  function mapSvg() {
    let seed = 7;
    const rnd = () => ((seed = (seed * 16807) % 2147483647) / 2147483647);
    let g = '<rect width="100" height="100" fill="#0d2a2b"/><path d="M0 62 C25 50 40 70 60 58 S90 48 100 55 V72 C80 66 62 80 40 74 S10 70 0 76Z" fill="#0b4a7a" opacity=".9"/><path d="M45 60h50" stroke="#cbd5e1" stroke-width=".8"/>';
    for (let k = 0; k < 70; k++) g += `<circle cx="${rnd() * 100}" cy="${rnd() * 100}" r="${1.2 + rnd() * 1.6}" fill="hsl(${120 + rnd() * 30} 55% ${18 + rnd() * 14}%)"/>`;
    for (let k = 0; k < 14; k++) {
      const x = 14 + rnd() * 52, h = 12 + rnd() * 24, w = 4 + rnd() * 4, y = 52 - rnd() * 6;
      g += `<rect x="${x}" y="${y - h}" width="${w}" height="${h}" fill="hsl(215 35% ${24 + rnd() * 14}%)"/>`;
      for (let wy = y - h + 2; wy < y - 1; wy += 2.6) g += `<rect x="${x + 1}" y="${wy}" width="${w - 2}" height=".9" fill="#fde68a" opacity=".8"/>`;
    }
    return `<svg class="map" viewBox="0 0 100 100" preserveAspectRatio="xMidYMid slice" aria-hidden="true">${g}</svg>`;
  }
  function dieHtml(id) {
    const P = { 1: [5], 2: [1, 9], 3: [1, 5, 9], 4: [1, 3, 7, 9], 5: [1, 3, 5, 7, 9], 6: [1, 3, 4, 6, 7, 9] };
    const face = (v) => `<b class="f${v}">${P[v].map((p) => `<i style="grid-area:${Math.ceil(p / 3)}/${((p - 1) % 3) + 1}"></i>`).join('')}</b>`;
    return `<div class="die" id="${id}">${[1, 2, 3, 4, 5, 6].map(face).join('')}</div>`;
  }
  function centerEl() {
    const c = document.createElement('div');
    c.className = 'center';
    c.innerHTML = mapSvg() +
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
  async function walk(p, from, to) {
    const N = state.board.length, i = state.players.findIndex((x) => x.id === p.id);
    for (let pos = from; pos !== to;) { pos = (pos + 1) % N; placePawn(p, pos, i); await sleep(200); }
  }

  // ---------- render ----------
  function render() {
    $('players').innerHTML = state.players.map((p) =>
      `<div class="pl${p.id === state.current_turn ? ' turn' : ''}${p.bankrupt ? ' out' : ''}" style="--c:${esc(p.color)}"><div class="av">${use('user')}</div><div><b>${esc(p.name)}${p.id === meId ? ' (você)' : ''}</b><span>${brl(p.balance)}</span></div>${p.id === state.current_turn ? '<span class="crown" aria-label="Vez de jogar">●</span>' : ''}</div>`).join('');
    $('cityName').textContent = state.city || 'Sua cidade';
    const mine = state.current_turn === meId && state.status === 'in_progress';
    const cur = state.players.find((p) => p.id === state.current_turn);
    $('statusText').textContent = state.status === 'finished' ? `Fim! ${state.winner || ''} venceu` : mine ? 'Sua vez' : `Vez de ${cur ? cur.name : '…'}`;
    $('btnRoll').disabled = busy || !state.can_roll;
    $('btnBuy').disabled = busy || !state.can_buy;
    $('btnEnd').disabled = busy || !state.can_end_turn;
    [$('btnBuild'), $('btnTrade')].forEach((b) => { b.disabled = true; b.title = 'Em breve'; });
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
      const d = $(id), [rx, ry] = FACE[dice[k]] || FACE[1];
      d.classList.remove('roll'); void d.offsetWidth; d.classList.add('roll');
      d.style.transform = `rotateX(${rx + 720 + k * 360}deg) rotateY(${ry + 720}deg)`;
    });
    await sleep(1000);
  }

  // apply a fresh room payload: animate pawn moves, then redraw
  async function commit(r) {
    raw = r;
    const next = norm(r), old = new Map(prevPos);
    if (!boardBuilt || $('board').children.length < next.board.length) { state = next; buildBoard(); }
    state = next;
    for (const p of state.players) {
      const from = old.get(p.id);
      if (from !== undefined && from !== p.position && !p.bankrupt) await walk(p, from, p.position);
      else if (!pawns.has(p.id)) placePawn(p, p.position, state.players.indexOf(p));
      prevPos.set(p.id, p.position);
    }
    render();
  }

  // ---------- game actions ----------
  async function run(fn) {
    if (busy) return;
    busy = true; render();
    try { await fn(); } catch (e) { toast(e.message); }
    busy = false; if (state) render();
  }
  const die = () => 1 + Math.floor(Math.random() * 6); // backend takes the dice values from the client
  const act = (name, extra) => api('/api/banco3d/room/' + name, { room_id: roomId, player_id: meId, ...extra });

  $('btnRoll').onclick = () => run(async () => {
    const r = await act('roll', { d1: die(), d2: die() });
    const d = r.dice || r.result || {};
    const dice = [d.die1 || (d.dice && d.dice[0]) || 1, d.die2 || (d.dice && d.dice[1]) || 1];
    await rollDice(dice);
    await commit(r.game ? r : await getRoom(roomId));
    const me = state.players.find((p) => p.id === meId), t = state.board[me.position];
    toast(`Tirou ${dice[0] + dice[1]}. ${t.name}.` + (state.can_buy ? ` Comprar por ${brl(t.price)}?` : ''));
  });
  $('btnBuy').onclick = () => run(async () => { const r = await act('buy'); await commit(r.game ? r : await getRoom(roomId)); toast('Comprado!'); });
  $('btnEnd').onclick = () => run(async () => { const r = await act('pass'); await commit(r.game ? r : await getRoom(roomId)); });
  $('btnMap').onclick = () => { const f = $('table').classList.toggle('flat'); $('btnMap').setAttribute('aria-pressed', f); setTimeout(placeAll, 720); };

  function sheet(title, html) { $('sheetTitle').textContent = title; $('sheetBody').innerHTML = html; $('sheet').hidden = false; }
  $('sheetClose').onclick = () => ($('sheet').hidden = true);
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') $('sheet').hidden = true; });
  function showTile(i) {
    const t = state.board[i], o = state.players.find((p) => p.id === t.owner_id);
    sheet(CORNER[t.type] || t.name, `${t.queen ? '<p>★ Ícone da cidade (votado)</p>' : ''}<p>${t.price ? 'Preço ' + brl(t.price) : ''}</p>${t.rent_base ? `<p>Aluguel ${brl(t.rent_base)}</p>` : ''}<p>${o ? 'Dono: ' + esc(o.name) : t.price ? 'Livre' : ''}</p>`);
  }

  // ---------- match config modal ----------
  const CFG_KEY = 'banco3d:config';
  const DEFAULT_CFG = { city: 'Curitiba, PR', players: 4, balance: 1500, bots: 0, difficulty: 'normal', turn: 30, bail: true, triple: true, freecard: true };
  let cfg = { ...DEFAULT_CFG };
  try { cfg = { ...cfg, ...JSON.parse(localStorage.getItem(CFG_KEY)) }; } catch { /* first run */ }
  let cfgOpener = null;
  const cf = () => $('cfgForm');
  function fillCfg() {
    const f = cf();
    f.city.value = $('fHome').city.value || cfg.city;
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
    cfg = { city: f.city.value.trim() || DEFAULT_CFG.city, players: +f.players.value, balance: Math.min(10000, Math.max(500, +f.balance.value || 1500)), bots: +f.bots.value, difficulty: f.difficulty.value, turn: +f.turn.value, bail: f.bail.checked, triple: f.triple.checked, freecard: f.freecard.checked };
    try { localStorage.setItem(CFG_KEY, JSON.stringify(cfg)); } catch { /* private mode */ }
    $('fHome').city.value = cfg.city;
    closeCfg();
  };
  const cfgPayload = () => ({ max_players: cfg.players, bots: { count: cfg.bots, difficulty: cfg.difficulty }, starting_balance: cfg.balance, turn_seconds: cfg.turn, rules: { jail_bail: cfg.bail, jail_triple_doubles: cfg.triple, get_out_free_card: cfg.freecard } });

  // ---------- lobby / voting / room flow ----------
  const setView = (v) => { $('start').dataset.view = v; $('start').hidden = v === 'game'; };
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
    const u = new URL(location.href); u.searchParams.set('sala', id); history.replaceState(null, '', u);
    startPolling();
  }

  $('fHome').onsubmit = async (e) => {
    e.preventDefault(); err();
    const f = e.target;
    try {
      const id = newCode();
      const c = await api('/api/banco3d/room/create', { room_id: id, city: f.city.value.trim(), config: cfgPayload() });
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
      if (/não encontrada|not found/i.test(x.message)) { err('Sala não existe mais.'); roomId = null; setView('home'); }
    }
    polling = false;
  }
  let timer;
  function startPolling() { clearInterval(timer); tick(); timer = setInterval(tick, 1000); }
  addEventListener('resize', () => state && placeAll());

  // ---------- boot: reconnect from ?sala= ----------
  (async () => {
    $('fHome').city.value = cfg.city;
    const id = (qs.get('sala') || '').toUpperCase();
    if (qs.get('config') === '1') openCfg();
    if (!id) return setView('home');
    $('joinCode').textContent = id; roomId = id;
    try {
      const st = await getRoom(id);
      $('joinCity').textContent = st.city;
      const saved = loadSaved(id);
      if (saved) await join(id, saved.name || 'Jogador'); else setView('join');
    } catch (x) { roomId = null; setView('home'); err(x.message); }
  })();

  // ---------- demo engine (?demo): in-memory banco3d backend for visual checks ----------
  const R = {};
  function demoApi(path, b) {
    const ok = (v) => Promise.resolve(structuredClone(v)), no = (m) => Promise.reject(new Error(m));
    const P = path.split('?')[0];
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
    const me = g.players.find((x) => x.id === b.player_id);
    if (P.endsWith('/room/roll')) {
      const d1 = 1 + Math.floor(Math.random() * 6), d2 = 1 + Math.floor(Math.random() * 6);
      me.position = (me.position + d1 + d2) % 40; g.dice_rolled_this_turn = true;
      return ok({ ...room, dice: { die1: d1, die2: d2 } });
    }
    if (P.endsWith('/room/buy')) { const t = room.board.tiles[me.position]; t.owner_id = me.id; me.money -= t.price; me.properties.push(t.index); return ok(room); }
    if (P.endsWith('/room/pass')) { g.players.slice(1).forEach((p) => { p.position = (p.position + 2 + Math.floor(Math.random() * 10)) % 40; }); g.dice_rolled_this_turn = false; return ok(room); }
    return no('Indisponível no modo demo');
  }
})();
