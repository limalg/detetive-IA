(() => {
  const $ = (id) => document.getElementById(id);
  const qs = new URLSearchParams(location.search);
  const API = qs.get('api') || '';
  const DEMO = qs.has('demo');
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const brl = (n) => 'R$ ' + Number(n).toLocaleString('pt-BR');
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
  const luckName = () => 'Sorte ou Revés'; // CHANCE and CHEST share one tile look
  // festival_until: ISO date or a turn number; unknown/absent = no festival
  const festOn = (f, turn) => !!f && (typeof f === 'string' ? Date.parse(f) > Date.now() : typeof f === 'number' ? f > (turn || 0) : !!f);
  function norm(r) {
    const g = r.game || {}, tiles = (g.board && g.board.tiles) || (r.board && r.board.tiles) || []; // game.board is the live one (owner_id); top-level board can be a stale copy
    const players = (g.players || []).map((p) => ({ id: p.id, name: p.name, color: p.color, balance: p.money, position: p.position, bankrupt: p.is_bankrupt, properties: p.properties || [] }));
    const board = tiles.map((t) => ({ position: t.index, type: TT[t.type] || t.type, deck: t.type, name: t.name, group: t.group, price: t.price, rent: t.rent || [], rent_base: (t.rent || [])[0] || 0, build_cost: t.house_cost, image_kind: t.image_kind, description: t.description, mortgage: t.mortgage_value, owner_id: t.owner_id, houses: t.houses >= 5 ? 0 : t.houses, hotel: t.houses >= 5, queen: t.is_queen, festival: festOn(t.festival_until, g.turn_index) || ((t.festival_multiplier || 1) > 1 && (t.festival_rounds || 0) > 0), champion: (t.championship_multiplier || 1) > 1 }));
    players.forEach((p) => { p.worth = p.balance + board.filter((t) => t.owner_id === p.id).reduce((s, t) => s + (t.price || 0) + (t.hotel ? 5 : t.houses || 0) * (t.build_cost || 0), 0); }); // net worth: cash + deeds + buildings
    const cur = g.current_turn_player_id, me = players.find((p) => p.id === meId);
    const mine = cur === meId && g.is_started && !g.is_over, rolled = !!g.dice_rolled_this_turn;
    const t = me && board[me.position];
    const buyable = t && t.price > 0 && !t.owner_id && ['PROPERTY', 'STATION', 'UTILITY'].includes(t.type);
    const win = players.find((p) => p.id === g.winner_id);
    return { city: r.city, board, players, current_turn: cur, status: g.is_over ? 'finished' : 'in_progress', winner: win && win.name, match_end: g.match_ends_at || r.match_ends_at || null, turn_start: g.turn_start_time, decision_deadline: g.decision_deadline || r.decision_deadline || null,
      can_roll: mine && !rolled, can_buy: mine && rolled && !!buyable, can_end_turn: mine && rolled,
      card: g.last_card_result && g.last_card_result.card ? { key: g.turn_index + ':' + g.last_card_result.card.id + ':' + g.last_card_result.player_id, deck: g.last_card_result.card.deck || g.last_card_result.card.type, title: g.last_card_result.card.title, delta: g.last_card_result.money_delta, jail: g.last_card_result.went_to_jail, who: g.last_card_result.player_id } : null };
  }

  // ---------- board ----------
  const STATION_ICONS = ['bolt', 'drop', 'wifi', 'train'];
  const CORNER = { START: 'Início', TRAVEL: 'Viagem', JAIL: 'Ilha Perdida', SPECIAL_TAX: 'Imposto Especial', PARKING: 'Campeonato', GO_TO_JAIL: 'Vá para a Ilha' };
  const CSUB = { START: 'Receba o salário', TRAVEL: 'World Tour', JAIL: 'Prisão', SPECIAL_TAX: 'Sorte ou imposto', PARKING: 'Visitante', GO_TO_JAIL: 'Direto para a prisão' };
  // short price for the tiles: R$ 60, R$ 400, R$ 1,2K
  const kshort = (n) => (n >= 1000 ? 'R$ ' + (n / 1000).toLocaleString('pt-BR', { maximumFractionDigits: 1 }) + 'K' : 'R$ ' + n);
  const side = (i, q) => (i < q ? 'B' : i < 2 * q ? 'L' : i < 3 * q ? 'T' : 'R');
  function cell(i, q) {
    const S = q + 1;
    if (i < q) return [S, S - i];
    if (i < 2 * q) return [S - (i - q), 1];
    if (i < 3 * q) return [1, i - 2 * q + 1];
    return [i - 3 * q + 1, S];
  }
  const use = (id) => `<svg aria-hidden="true"><use href="#i-${id}"/></svg>`;
  function tileIcon(t, n) {
    if (t.type === 'START') return 'arrow';
    if (t.type === 'JAIL') return 'island';
    if (t.type === 'TRAVEL') return 'plane';
    if (t.type === 'SPECIAL_TAX') return 'coin';
    if (t.type === 'PARKING') return 'trophy';
    if (t.type === 'GO_TO_JAIL') return 'plane';
    if (t.type === 'LUCK') return 'q';
    if (t.type === 'TAX') return 'coin';
    if (t.type === 'STATION' || t.type === 'UTILITY') return STATION_ICONS[n % 4];
    return 'house';
  }

  function buildBoard() {
    const board = $('board'), tiles = state.board, N = tiles.length, q = N / 4, S = q + 1;
    const tracks = `1.9fr repeat(${q - 1}, 1fr) 1.9fr`;
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
      const nm = CORNER[t.type] || (t.type === 'LUCK' ? luckName(t) : t.name);
      const icon = tileIcon(t, t.type === 'STATION' || t.type === 'UTILITY' ? sn++ : i);
      const own = ['PROPERTY', 'STATION', 'UTILITY'].includes(t.type);
      if (own) el.classList.add('ownable');
      el.innerHTML = corner ? `${use(icon)}<span class="nm">${esc(nm)}</span><small class="sb">${CSUB[t.type] || ''}</small>`
        : `${own ? '<i class="band"></i>' : ''}<span class="tx">${t.type === 'PROPERTY' ? '' : use(icon)}<span class="nm">${esc(nm)}</span>${t.price ? `<span class="pr">${kshort(t.price)}</span>` : ''}</span>${t.type === 'PROPERTY' ? '<span class="bld"></span>' : ''}${own ? '<i class="flag" aria-hidden="true"></i><i class="fl2" aria-hidden="true"></i>' : ''}${t.type === 'PROPERTY' ? '<i class="seal" title="Monopólio: aluguel dobrado" aria-hidden="true">×2</i>' : ''}`;
      el.setAttribute('aria-label', `${nm}${t.price ? ', ' + brl(t.price) : ''}`);
      el.onclick = () => showTile(i);
      board.appendChild(el);
    });
    board.appendChild(centerEl());
    pawns.clear(); prevPos.clear(); pawnPos.clear(); diceEls = null; boardBuilt = true;
    fitTiles();
  }

  // shrink each name until name + price fit inside the tile (no clipped text, price always visible)
  function fitTiles() {
    $('board').querySelectorAll('.tile').forEach((t) => {
      const nm = t.querySelector('.nm'), tx = t.querySelector('.tx') || t;
      if (!nm) return;
      nm.style.fontSize = '';
      let fs = parseFloat(getComputedStyle(nm).fontSize);
      const over = () => tx.scrollHeight > tx.clientHeight + 1 || tx.scrollWidth > tx.clientWidth + 1 || nm.scrollWidth > nm.clientWidth + 1;
      while (over() && fs > 4.5) { fs -= .5; nm.style.fontSize = fs + 'px'; }
    });
  }
  let fitT;
  addEventListener('resize', () => { clearTimeout(fitT); fitT = setTimeout(() => boardBuilt && fitTiles(), 150); });

  function cubeHtml() {
    const P = { 1: [5], 2: [1, 9], 3: [1, 5, 9], 4: [1, 3, 7, 9], 5: [1, 3, 5, 7, 9], 6: [1, 3, 4, 6, 7, 9] };
    const face = (v) => `<b class="f${v}">${P[v].map((p) => `<i style="grid-area:${Math.ceil(p / 3)}/${((p - 1) % 3) + 1}"></i>`).join('')}</b>`;
    return `<i class="dsh"></i><div class="dz"><div class="cube">${[1, 2, 3, 4, 5, 6].map(face).join('')}</div></div>`;
  }
  function centerEl() {
    const c = document.createElement('div');
    c.className = 'center';
    c.innerHTML =
      `<div class="clock" id="matchClock" role="timer" aria-label="Tempo restante da partida" hidden>${use('clock')}<b id="clockTxt">--:--</b></div>` +
      `<div class="logo"><svg viewBox="0 0 64 32" aria-hidden="true"><path d="M4 30V16l8-6v20M16 30V8l9-6v28M29 30V12l9-4v22M42 30V4l10 6v20M56 30V14l4 2v14" fill="none" stroke="currentColor" stroke-width="2.500"/></svg>BANCO IMOBILIÁRIO</div>` +
      `<div class="piles"><div class="pile" title="Sorte ou Revés">${use('q')}</div><div class="pile" title="Sorte ou Revés">${use('q')}</div></div>`;
    return c;
  }

  // ---------- pawns ----------
  const PAWN = '<svg viewBox="0 0 24 40" aria-hidden="true"><defs><linearGradient id="pwS"><stop offset="0" stop-color="#fff" stop-opacity=".45"/><stop offset=".45" stop-color="#fff" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity=".4"/></linearGradient><radialGradient id="pwH" cx=".35" cy=".3" r=".75"><stop offset="0" stop-color="#fff" stop-opacity=".85"/><stop offset=".35" stop-color="#fff" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity=".45"/></radialGradient></defs>'
    + '<ellipse class="c" cx="12" cy="35" rx="10" ry="3.6"/><path class="c" d="M6.500 34C8.500 27 10 22 10.200 17h3.600c.2 5 1.700 10 3.700 17z"/><ellipse class="c" cx="12" cy="17" rx="4.800" ry="1.800"/><circle class="c" cx="12" cy="10" r="6.500"/>'
    + '<ellipse fill="url(#pwS)" cx="12" cy="35" rx="10" ry="3.600"/><path fill="url(#pwS)" d="M6.500 34C8.500 27 10 22 10.200 17h3.600c.2 5 1.700 10 3.700 17z"/><circle fill="url(#pwH)" cx="12" cy="10" r="6.500"/></svg>';
  const pawnPos = new Map();
  function placePawn(p, pos) {
    let el = pawns.get(p.id);
    if (!el) {
      el = document.createElement('div');
      el.className = 'pawn';
      el.innerHTML = `<i>${PAWN}</i>`;
      el.style.setProperty('--c', p.color);
      $('board').appendChild(el);
      pawns.set(p.id, el);
    }
    pawnPos.set(p.id, pos);
    layoutPawns();
  }
  // pawn centre = tile centre; pawns sharing a tile spread on a small circle that stays inside it
  function layoutPawns() {
    const at = new Map();
    pawns.forEach((el, id) => { const pos = pawnPos.get(id); if (pos !== undefined) (at.get(pos) || at.set(pos, []).get(pos)).push(el); });
    at.forEach((els, pos) => {
      const t = $('board').children[pos], n = els.length, r = n > 1 ? Math.min(t.offsetWidth, t.offsetHeight) * .2 : 0, a0 = -Math.PI / 2 + 'LTRB'.indexOf(t.dataset.side) * Math.PI / 2;
      els.forEach((el, k) => {
        const a = a0 + k * 2 * Math.PI / n;
        el.style.transform = `translate(${t.offsetLeft + t.offsetWidth / 2 + r * Math.cos(a)}px,${t.offsetTop + t.offsetHeight / 2 + r * Math.sin(a)}px)`;
        el.style.setProperty('--k', n > 1 ? .78 : 1);
      });
    });
  }
  function placeAll() {
    if (!boardBuilt) return;
    state.players.forEach((p) => { if (!p.bankrupt) placePawn(p, p.position); prevPos.set(p.id, p.position); });
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
    // same keyframes as .tile.land: the pawn rides the tile up (own `translate` prop) and gets a small extra hop of its own
    const lift = parseFloat(getComputedStyle($('board')).fontSize) * 1.6;
    pawns.forEach((el, id) => {
      if (pawnPos.get(id) !== pos) return;
      el.animate([{ translate: '0 0 0' }, { translate: `0 0 ${lift}px`, offset: .2 }, { translate: '0 0 0' }], { duration: 1000, easing: 'cubic-bezier(.25,.1,.25,1)' });
      el.firstChild.animate([{ translate: '0 0' }, { translate: '0 0', offset: .2 }, { translate: `0 ${-lift * 1.4}px`, offset: .38 }, { translate: '0 0', offset: .56 }, { translate: '0 0' }], { duration: 1000, easing: 'ease-out' });
    });
    setTimeout(() => t.classList.remove('land'), 1100);
  }
  async function walk(p, from, to) {
    const N = state.board.length;
    for (let pos = from; pos !== to;) { pos = (pos + 1) % N; placePawn(p, pos); hop(p); camera(pos); await sleep(200); }
    land(to);
  }

  // ---------- motion toggle (default: max, ignores the OS preference) + camera that follows the turn ----------
  let motion = 'max';
  try { if (localStorage.getItem('banco3d:motion') === 'reduced') motion = 'reduced'; } catch { /* storage blocked */ }
  const maxMotion = () => motion === 'max';
  // screen/modal changes: View Transitions API where available (CSS animation fallback otherwise); fn runs either way
  const vt = (fn) => { if (document.startViewTransition && maxMotion()) document.startViewTransition(fn); else fn(); };
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

  // ---------- event card art (assets/cards): image_kind -> file ----------
  const CARDIMG = {};
  const cardsReady = (async () => {
    try { const r = await fetch('assets/cards/manifest.json'); if (r.ok) Object.entries((await r.json()).cards || {}).forEach(([k, f]) => { CARDIMG[k] = 'assets/cards/' + f; }); } catch { /* no manifest: cards show without art */ }
  })();

  // ---------- players panel: persistent rows, balance tween, floating deltas ----------
  const rows = new Map(), bal = new Map();
  let fxSrc = new Map(); // player id -> element the coins should leave from (e.g. the Saída tile)
  function tweenTo(id, el, to) {
    const b = bal.get(id), from = b.shown, t0 = performance.now(), dur = maxMotion() ? 1000 : 1;
    b.target = to; cancelAnimationFrame(b.raf);
    el.classList.remove('up', 'down'); el.classList.add(to > from ? 'up' : 'down');
    clearTimeout(b.fx); b.fx = setTimeout(() => el.classList.remove('up', 'down'), dur + 700);
    const step = (now) => {
      const k = Math.min(1, (now - t0) / dur);
      b.shown = Math.round(from + (to - from) * (1 - Math.pow(1 - k, 3)));
      el.textContent = brl(b.shown);
      if (k < 1) b.raf = requestAnimationFrame(step);
    };
    b.raf = requestAnimationFrame(step);
  }
  // coins and bills arcing from `from` to `to` (viewport elements)
  function coins(from, to, n) {
    if (!maxMotion() || !from || !to) return;
    const a = from.getBoundingClientRect(), b = to.getBoundingClientRect();
    const x0 = a.left + a.width / 2, y0 = a.top + a.height / 2, x1 = b.left + 40, y1 = b.top + b.height / 2;
    for (let k = 0; k < n; k++) {
      const c = document.createElement('i'), bill = k % 3 === 2, jx = (Math.random() - .5) * 60, arc = -70 - Math.random() * 60;
      c.className = 'coin' + (bill ? ' bill' : ''); c.textContent = bill ? '' : 'R$';
      document.body.appendChild(c);
      c.animate([
        { transform: `translate(${x0}px,${y0}px) scale(.4) rotateY(0deg)`, opacity: 0 },
        { transform: `translate(${(x0 + x1) / 2 + jx}px,${(y0 + y1) / 2 + arc}px) scale(1.1) rotateY(540deg)`, opacity: 1, offset: .45 },
        { transform: `translate(${x1}px,${y1}px) scale(.6) rotateY(1080deg)`, opacity: .9 },
      ], { duration: 900, delay: k * 70, easing: 'cubic-bezier(.4,0,.2,1)', fill: 'both' }).onfinish = () => c.remove();
    }
  }
  const wealthOf = (p) => p.balance + state.board.reduce((n, t) => n + (t.owner_id === p.id ? t.price || 0 : 0), 0);
  function renderPlayers() {
    const box = $('players'), W = new Map(state.players.map((p) => [p.id, wealthOf(p)])), top = Math.max(1, ...W.values());
    const gains = [], losses = [], src = fxSrc; fxSrc = new Map();
    state.players.forEach((p) => {
      let r = rows.get(p.id);
      if (!r) {
        r = document.createElement('div');
        r.innerHTML = `<div class="av">${use('user')}</div><div class="who"><b></b><span class="bal"></span></div><span class="crown" aria-label="Vez de jogar">●</span><div class="wealth"><div class="bar"><i></i></div><small></small></div><div class="pp"></div>`;
        rows.set(p.id, r); box.appendChild(r);
        bal.set(p.id, { shown: p.balance, target: p.balance, raf: 0 });
        r.querySelector('.bal').textContent = brl(p.balance);
      }
      r.className = 'pl' + (p.id === state.current_turn ? ' turn' : '') + (p.bankrupt ? ' out' : '') + (r.classList.contains('gain') ? ' gain' : '') + (r.classList.contains('loss') ? ' loss' : '');
      r.style.setProperty('--c', p.color);
      r.querySelector('b').textContent = p.name + (p.id === meId ? ' (você)' : '');
      r.querySelector('.crown').hidden = p.id !== state.current_turn;
      r.querySelector('.wealth i').style.width = (W.get(p.id) / top * 100) + '%';
      r.querySelector('.wealth small').textContent = 'Patrimônio ' + brl(W.get(p.id));
      r.querySelector('.pp').innerHTML = state.board.filter((t) => t.owner_id === p.id).sort((x, y) => String(x.group || '~').localeCompare(String(y.group || '~')))
        .map((t) => `<i title="${esc(t.name)}" style="background:${t.type === 'PROPERTY' ? `var(--g-${t.group})` : '#7dd3fc'}"></i>`).join('');
      const b = bal.get(p.id), d = p.balance - b.target;
      if (d) (d > 0 ? gains : losses).push({ p, r, d });
    });
    [...gains, ...losses].forEach(({ p, r, d }) => {
      const f = document.createElement('span');
      f.className = 'fly ' + (d > 0 ? 'up' : 'down'); f.textContent = (d > 0 ? '+' : '−') + brl(Math.abs(d));
      r.appendChild(f); setTimeout(() => f.remove(), 1800);
      const cls = d > 0 ? 'gain' : 'loss';
      r.classList.remove('gain', 'loss'); void r.offsetWidth; r.classList.add(cls); setTimeout(() => r.classList.remove(cls), 1500);
      tweenTo(p.id, r.querySelector('.bal'), p.balance);
    });
    const center = $('board').querySelector('.center'), n = (d) => Math.min(9, 3 + Math.floor(Math.abs(d) / 100));
    gains.forEach(({ p, r, d }, k) => {
      const payer = src.get(p.id) || (losses[k] || losses[0] || {}).r || center;
      coins(payer, r, n(d));
    });
    if (!gains.length) losses.forEach(({ r, d }) => coins(r, center, n(d)));
  }

  // ---------- my properties: vertical strip of group-coloured squares; hover = summary, click = full card ----------
  const rentNow = (t) => (t.type === 'PROPERTY' && t.rent.length > 5 ? t.rent[t.hotel ? 5 : t.houses] : t.rent_base) || 0;
  let mineSig = '', mineSet = new Set();
  function renderMine() {
    const mine = state.board.filter((t) => t.owner_id === meId).sort((x, y) => String(x.group || '~').localeCompare(String(y.group || '~')) || x.position - y.position);
    const sig = mine.map((t) => `${t.position}:${t.houses}:${t.hotel}:${canBuild(t) ? 1 : 0}`).join('|');
    $('myProps').dataset.empty = mine.length ? '' : '1';
    if (sig === mineSig) return;
    mineSig = sig;
    const prev = mineSet; mineSet = new Set(mine.map((t) => t.position));
    $('myList').innerHTML = mine.map((t) => {
      const lv = t.hotel ? '★' : t.houses || '';
      return `<button type="button" class="mp${prev.size && !prev.has(t.position) ? ' new' : ''}" data-i="${t.position}" style="--gc:${t.type === 'PROPERTY' ? `var(--g-${t.group})` : '#5b6b82'}" aria-label="${esc(t.name)}, ${brl(t.price)}${lv ? ', ' + (t.hotel ? 'hotel' : t.houses + ' casas') : ''}">${t.type === 'PROPERTY' ? '' : use(tileIcon(t, t.position))}${lv ? `<em>${lv}</em>` : ''}${canBuild(t) ? '<span class="mpb" title="Construir" aria-hidden="true">+</span>' : ''}</button>`;
    }).join('');
    tipHide();
  }
  const tip = $('propTip');
  function tipShow(btn) {
    const t = state.board[+btn.dataset.i];
    tip.innerHTML = `<b>${esc(t.name)}</b><span>Preço <i>${brl(t.price)}</i></span><span>Aluguel <i>${brl(rentNow(t))}</i></span><span>Casas <i>${t.hotel ? 'Hotel' : t.houses}</i></span>`;
    tip.hidden = false;
    const r = btn.getBoundingClientRect(), w = tip.offsetWidth, h = tip.offsetHeight;
    tip.style.left = Math.max(8, r.left - w - 10) + 'px'; tip.style.top = Math.min(innerHeight - h - 8, Math.max(8, r.top + r.height / 2 - h / 2)) + 'px';
  }
  function tipHide() { tip.hidden = true; }
  const mpBtn = (e) => e.target.closest('.mp');
  $('myList').addEventListener('mouseover', (e) => { const b = mpBtn(e); if (b) tipShow(b); });
  $('myList').addEventListener('mouseout', (e) => { if (mpBtn(e)) tipHide(); });
  $('myList').addEventListener('focusin', (e) => { const b = mpBtn(e); if (b) tipShow(b); });
  $('myList').addEventListener('focusout', tipHide);
  $('myList').addEventListener('click', (e) => { const b = mpBtn(e); if (!b) return; tipHide(); if (e.target.closest('.mpb')) buildAt(+b.dataset.i); else showTile(+b.dataset.i); });

  function buildWhy(t) {
    const me = state.players.find((p) => p.id === meId);
    if (!me || t.type !== 'PROPERTY' || t.owner_id !== meId) return 'Só em propriedades suas.';
    if (state.status !== 'in_progress' || state.current_turn !== meId) return 'Só no seu turno.';
    if (!state.board.filter((x) => x.type === 'PROPERTY' && x.group === t.group).every((x) => x.owner_id === meId)) return 'Precisa do grupo de cor completo.';
    if (t.hotel) return 'Já tem hotel.';
    if (me.balance < (t.build_cost || 0)) return 'Saldo insuficiente.';
    return '';
  }
  const canBuild = (t) => !buildWhy(t);
  const buildLabel = (t) => `Construir ${t.houses === 4 ? 'hotel' : 'casa'} · ${brl(t.build_cost || 0)}`;
  const buildAt = (i) => run(async () => {
    try {
      const r = await act('build', { tile_index: i });
      await commit(r.game ? r : await getRoom(roomId)); // render() sees the higher level and plays the 'rise' animation
      toast(`${state.board[i].name}: ${state.board[i].hotel ? 'hotel' : state.board[i].houses + ' casa(s)'}`);
    } catch (x) { toast(x.message); }
  });

  // one 3D building: front / right / left walls + roof lid, built from the footprint up
  const bxHtml = (cls, rise) => `<i class="bx ${cls}${rise ? ' rise' : ''}"><b class="ft"></b><b class="rt"></b><b class="lf"></b><b class="tp"></b></i>`;

  // ---------- render ----------
  function render() {
    renderPlayers();
    renderMine();
    pawns.forEach((el, id) => el.classList.toggle('turn', id === state.current_turn));
    followTurn();
    $('cityName').textContent = state.city || 'Sua cidade';
    const mine = state.current_turn === meId && state.status === 'in_progress';
    const cur = state.players.find((p) => p.id === state.current_turn);
    $('statusText').textContent = state.status === 'finished' ? `Fim! ${state.winner || ''} venceu` : mine ? 'Sua vez' : `Vez de ${cur ? cur.name : '…'}`;
    $('btnRoll').disabled = busy || !state.can_roll;
    $('btnBuy').disabled = !state.can_buy; // stays armed while the pawn is still walking
    $('btnEnd').disabled = busy || !state.can_buy;
    $('btnBuild').disabled = busy || !state.board.some(canBuild); $('btnBuild').title = $('btnBuild').disabled ? 'Precisa do grupo de cor completo, no seu turno' : '';
    $('btnTrade').disabled = busy || state.status !== 'in_progress' || state.players.filter((p) => !p.bankrupt).length < 2;
    const gOwn = {}, mono = {}; // monopoly = every tile of the group has the same owner
    state.board.forEach((t) => { if (t.type === 'PROPERTY') (gOwn[t.group] = gOwn[t.group] || []).push(t.owner_id || ''); });
    Object.entries(gOwn).forEach(([g, l]) => { mono[g] = !!l[0] && l.every((x) => x === l[0]); });
    state.board.forEach((t, i) => {
      const el = $('board').children[i];
      const o = state.players.find((p) => p.id === t.owner_id);
      el.classList.toggle('owned', !!o);
      if (o) el.style.setProperty('--oc', o.color);
      const bl = el.querySelector('.bld');
      if (bl) {
        const lv = t.hotel ? 5 : t.houses || 0, prev = +bl.dataset.lv || 0;
        if (bl.dataset.lv !== String(lv)) {
          const fresh = bl.dataset.init && lv > prev && maxMotion(); // first paint and reloads don't animate
          bl.dataset.lv = lv; bl.dataset.init = 1;
          bl.innerHTML = lv === 5 ? bxHtml('hotel', fresh) : Array.from({ length: lv }, (_, k) => bxHtml('', fresh && k >= prev)).join('');
        }
      }
      el.classList.toggle('mono', t.type === 'PROPERTY' && mono[t.group]);
      el.classList.toggle('fest', !!t.festival);
      el.classList.toggle('champ', !!t.champion);
      el.querySelector('.nm').textContent = CORNER[t.type] || (t.type === 'LUCK' ? luckName(t) : t.name);
    });
  }

  let toastT;
  function toast(msg) {
    const t = $('toast');
    t.textContent = msg; t.hidden = false;
    clearTimeout(toastT); toastT = setTimeout(() => (t.hidden = true), 3600);
  }

  // ---------- dice: real cubes thrown from the thrower's edge, bouncing and rolling to a random spot on the table ----------
  const FACE = { 1: [0, 0], 2: [0, -90], 3: [-90, 0], 4: [90, 0], 5: [0, 90], 6: [0, 180] }; // rotateX/rotateY that put face v on top
  let diceEls = null; const diceRest = [];
  const rnd = (a, b) => a + Math.random() * (b - a);
  function ensureDice() {
    if (diceEls && diceEls[0].isConnected) return diceEls;
    diceEls = [0, 1].map(() => { const w = document.createElement('div'); w.className = 'dice-w'; w.hidden = true; w.innerHTML = cubeHtml(); $('board').appendChild(w); return w; });
    return diceEls;
  }
  // dice rest as board fractions, so a resize keeps them on the same spot
  function layoutDice() {
    if (!diceEls || !diceEls[0].isConnected) return;
    const b = $('board'), d = b.offsetWidth * .042;
    diceEls.forEach((w, k) => { const r = diceRest[k]; if (!r) return; w.style.setProperty('--d', d + 'px'); w.style.transform = `translate(${r.fx * b.offsetWidth}px,${r.fy * b.offsetHeight}px)`; });
  }
  const T_IMP = [.3, .58, .8, .94], H_BOUNCE = [3, 1.5, .6, .2]; // impact times and bounce heights (in die sizes)
  const heightAt = (t) => {
    if (t <= T_IMP[0]) return H_BOUNCE[0] * (1 - (t / T_IMP[0]) ** 2);
    for (let i = 1; i < 4; i++) if (t <= T_IMP[i]) { const u = (t - T_IMP[i - 1]) / (T_IMP[i] - T_IMP[i - 1]); return H_BOUNCE[i] * 4 * u * (1 - u); }
    return 0;
  };
  async function rollDice(dice, who) {
    const b = $('board'), c = b.querySelector('.center'), W = b.offsetWidth, H = b.offsetHeight, d = W * .042, els = ensureDice();
    const x0 = c.offsetLeft + d, x1 = c.offsetLeft + c.offsetWidth - d, y0 = c.offsetTop + d, y1 = c.offsetTop + c.offsetHeight * .62; // above the card piles
    let tg;
    for (let n = 0; n < 40; n++) { tg = [0, 1].map(() => [rnd(x0, x1), rnd(y0, y1)]); if (Math.hypot(tg[0][0] - tg[1][0], tg[0][1] - tg[1][1]) > d * 2.5) break; }
    const k0 = state.players.findIndex((p) => p.id === who), sd = who === meId ? 'B' : ['L', 'T', 'R'][Math.max(0, k0) % 3], o = rnd(.35, .65);
    els.forEach((w, k) => {
      const [rx, ry] = FACE[dice[k]] || FACE[1], yaw = rnd(0, 360), [x, y] = tg[k], sp = (k ? 1 : -1) * d * .8;
      const [sx, sy] = sd === 'B' ? [W * o + sp, H + d * 2] : sd === 'T' ? [W * o + sp, -d * 2] : sd === 'L' ? [-d * 2, H * o + sp] : [W + d * 2, H * o + sp];
      const dz = w.querySelector('.dz'), cube = w.querySelector('.cube'), sh = w.querySelector('.dsh');
      diceRest[k] = { fx: x / W, fy: y / H };
      w.hidden = false; w.style.setProperty('--d', d + 'px');
      w.style.transform = `translate(${x}px,${y}px)`; dz.style.transform = `translateZ(${d / 2}px)`; cube.style.transform = `rotateZ(${yaw}deg) rotateX(${rx}deg) rotateY(${ry}deg)`;
      if (!maxMotion()) return;
      const spin = [0, 1, 2].map(() => (Math.random() < .5 ? -1 : 1) * 360 * rnd(1.5, 3)), N = 40, wk = [], zk = [], ck = [], sk = [];
      for (let i = 0; i <= N; i++) {
        const t = i / N, p = 1 - (1 - t) ** 2.4, z = heightAt(t) * d, e = 1 - p;
        wk.push({ transform: `translate(${sx + (x - sx) * p}px,${sy + (y - sy) * p}px)` });
        zk.push({ transform: `translateZ(${d / 2 + z}px)` });
        ck.push({ transform: `rotateZ(${yaw + e * spin[0]}deg) rotateX(${rx + e * spin[1]}deg) rotateY(${ry + e * spin[2]}deg)` });
        sk.push({ transform: `translateZ(.5px) scale(${1 / (1 + z / d * .7)})`, opacity: .45 / (1 + z / d) });
      }
      const opt = { duration: 1200, easing: 'linear' };
      w.animate(wk, opt); dz.animate(zk, opt); cube.animate(ck, opt); sh.animate(sk, opt);
    });
    await sleep(maxMotion() ? 1200 : 1);
  }

  // apply a fresh room payload: roll dice + walk pawns for every mover, then redraw
  let seenCard; // undefined until the first state, so a reload doesn't replay an old card
  // Sorte/Revés reveal: queued so back-to-back cards (bots) play one after another; closes by itself or on click
  const cardQ = []; let cardBusy = false, cardClose = null, cardEvSeen = false;
  async function pumpCards() {
    if (cardBusy) return;
    cardBusy = true;
    const box = $('cardFx'), card = $('cfxCard');
    while (cardQ.length) {
      const c = cardQ.shift();
      if (c.destroy != null) { camera(c.destroy); crumble(c.destroy); await sleep(maxMotion() ? 1500 : 1); continue; }
      const p = state.players.find((x) => x.id === c.who), amt = Number(c.amount) || 0;
      $('cfxDeck').textContent = 'Sorte ou Revés';
      $('cfxWho').textContent = `Sorte ou Revés${p ? ' de ' + (p.id === meId ? 'você' : p.name) : ''}`;
      const art = CARDIMG[c.kind] || '', img = $('cfxImg'); img.hidden = !art; if (art) img.src = art;
      $('cfxTitle').textContent = c.title || ''; $('cfxText').textContent = c.text || '';
      const fx = $('cfxFx'); fx.className = amt > 0 ? 'up' : amt < 0 ? 'down' : '';
      fx.textContent = c.jail ? 'Vá para a Cadeia' : amt ? `${amt > 0 ? '+' : '−'}${brl(Math.abs(amt))}` : '';
      card.className = 'cfx-card chance' + (art ? ' art' : '');
      box.hidden = false; void card.offsetWidth;
      card.classList.add('in'); await sleep(maxMotion() ? 450 : 1); card.classList.add('flip');
      await new Promise((ok) => { cardClose = ok; setTimeout(ok, 3300); });
      cardClose = null; card.classList.add('out'); await sleep(maxMotion() ? 320 : 1); box.hidden = true;
    }
    cardBusy = false;
  }
  // close: X button, Esc, click outside the card or on it. Local only: other clients keep showing their own copy
  const dismissCard = () => cardClose && cardClose();
  $('cfxCard').onclick = $('cfxX').onclick = dismissCard;
  $('cardFx').addEventListener('click', (e) => { if (!e.target.closest('#cfxCard')) dismissCard(); });
  addEventListener('keydown', (e) => { if (e.key === 'Escape' && cardClose) { e.stopPropagation(); dismissCard(); } }, true);
  const cardShown = new Set(); // events already queued, so a dismissed card never comes back
  // a destroyed tile shakes, sags and drops dust
  function crumble(pos) {
    const t = $('board').children[pos];
    if (!t || !maxMotion()) return;
    const sh = (x, r) => ({ transform: `translateX(${x}em) rotate(${r}deg)` });
    t.animate([sh(0, 0), sh(-.5, -2), sh(.5, 2), sh(-.6, -2.5), sh(.6, 2.5), sh(-.4, -1.5), sh(.3, 1), { transform: 'translateY(.35em) scale(.95,.88) rotate(1.5deg)', filter: 'saturate(.5) brightness(.85)', offset: .82 }, { transform: 'none', filter: 'none' }], { duration: 1400, easing: 'ease-in-out' });
    for (let k = 0; k < 7; k++) {
      const d = document.createElement('i');
      d.className = 'dust'; d.style.left = 10 + Math.random() * 80 + '%'; d.style.top = 30 + Math.random() * 50 + '%';
      t.appendChild(d);
      d.animate([{ opacity: 0, transform: 'translateY(0) scale(.4)' }, { opacity: .8, offset: .25 }, { opacity: 0, transform: `translate(${(Math.random() - .5) * 1.5}em,-2.2em) scale(1.6)` }], { duration: 1100, delay: 250 + k * 60, easing: 'ease-out', fill: 'both' }).onfinish = () => d.remove();
    }
  }
  if (DEMO) window.__demo = { own: (g) => demoApi('/room/__own', { room_id: roomId, player_id: meId, group: g }), card: (c) => { cardQ.push(c); pumpCards(); }, destroy: (pos) => { cardQ.push({ destroy: pos }); pumpCards(); } };
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
    if (!roomId) return ''; // left the match while a request was in flight
    raw = r;
    const next = norm(r), old = new Map(prevPos);
    await Promise.all([propsReady, cardsReady]);
    if (!boardBuilt || $('board').children.length < next.board.length) { state = next; buildBoard(); }
    state = next;
    const evs = freshEvents(r), diceBy = new Map(), N = state.board.length;
    (evs || []).filter((e) => e.type === 'roll' && e.dice).forEach((e) => diceBy.set(e.player_id, e.dice));
    for (const p of state.players) {
      const from = old.get(p.id);
      if (from !== undefined && from !== p.position && !p.bankrupt) {
        if (!(rolled && p.id === meId)) {
          const dice = diceBy.get(p.id) || guessDice(from, p.position, N);
          if (dice) { if (p.id !== meId) toast(`${p.name} tirou ${dice[0] + dice[1]}`); await rollDice(dice, p.id); }
        }
        if ((p.position - from + N) % N > 14) { placePawn(p, p.position); camera(p.position); land(p.position); } // card/jail teleport
        else await walk(p, from, p.position);
      } else if (!pawns.has(p.id)) placePawn(p, p.position);
      prevPos.set(p.id, p.position);
    }
    (evs || []).filter((e) => e.type === 'festival' || e.type === 'championship').forEach((e) => { const t = $('board').children[e.tile ?? e.position ?? e.to]; if (t && maxMotion()) { t.classList.remove('fest-pop'); void t.offsetWidth; t.classList.add('fest-pop'); setTimeout(() => t.classList.remove('fest-pop'), 1600); } });
    const salary = (evs || []).filter((e) => e.type === 'salary'), cardEvs = (evs || []).filter((e) => e.type === 'card');
    salary.forEach((e) => fxSrc.set(e.player_id, $('board').children[0]));
    render();
    checkOffer();
    autoTurn();
    const over = (evs || []).find((e) => e.type === 'game_over');
    if (over || state.status === 'finished') showFinal(over);
    if (cardEvs.length) cardEvSeen = true; // server emits card events: last_card_result is then only a stale duplicate
    cardEvs.filter((e) => !cardShown.has(evKey(e)) && cardShown.add(evKey(e))).forEach((e) => cardQ.push({ kind: e.image_kind || e.kind, deck: e.deck, title: e.title, text: e.text, amount: e.amount ?? e.value, jail: e.went_to_jail || /jail|prisao|cadeia/i.test(e.effect || ''), who: e.player_id }));
    const c = state.card, fresh = c && seenCard !== undefined && c.key !== seenCard;
    seenCard = c ? c.key : (seenCard === undefined ? '' : seenCard);
    (evs || []).filter((e) => e.type === 'destroy').forEach((e) => { const pos = e.tile ?? e.to ?? e.position; if (pos != null) cardQ.push({ destroy: +pos }); });
    if (fresh && !cardEvSeen) cardQ.push({ deck: c.deck, title: c.title, text: '', amount: c.delta, jail: c.jail, who: c.who });
    if (cardQ.length) pumpCards();
    if (!evs) return '';
    const evMsg = (e) => e.message || (e.type === 'salary' ? `${(state.players.find((x) => x.id === e.player_id) || {}).name || 'Jogador'} recebeu ${brl(e.amount ?? e.value ?? 0)} ao passar pela Saída` : ['festival', 'championship', 'destroy'].includes(e.type) ? e.text || '' : '');
    const idle = evs.filter((e) => (e.type === 'timeout' || (e.type === 'pass' && /inativ/i.test(e.text || '')))).map((e) => `${e.player_id === meId ? 'Você' : e.player_name || 'Jogador'} passou por inatividade`);
    const msgs = evs.filter((e) => !['roll', 'pass', 'card', 'move', 'timeout'].includes(e.type)).map(evMsg).concat(idle).filter(Boolean);
    if (msgs.length && !quiet) toast(msgs[msgs.length - 1]);
    return msgs.join(' · ');
  }

  // ---------- automatic turn: 8s window to buy, then the turn passes by itself ----------
  let autoKey = null, autoT = null, cdI = null;
  function stopAuto() { clearTimeout(autoT); clearInterval(cdI); autoT = cdI = autoKey = null; $('btnEnd').hidden = true; $('buyCd').hidden = true; }
  // `ti` = turn the pass was meant for: if the server already moved on (even back to me), don't pass the new turn
  async function passTurn(msg, ti = raw.game.turn_start_time) {
    if (busy) return void setTimeout(() => passTurn(msg, ti), 400);
    await run(async () => {
      const fresh = await getRoom(roomId);
      if (fresh.game.current_turn_player_id !== meId || fresh.game.turn_start_time !== ti || !fresh.game.dice_rolled_this_turn) { await commit(fresh); return; }
      if (msg) toast(msg);
      const r = await act('pass');
      await commit(r.game ? r : await getRoom(roomId));
    });
  }
  function autoTurn() {
    const mine = state.current_turn === meId && state.status === 'in_progress';
    const ti = raw.game.turn_start_time, key = `${state.current_turn}:${ti}:${state.can_buy ? 'b' : state.can_end_turn ? 'p' : 'r'}`;
    if (key === autoKey) return;
    stopAuto(); autoKey = key;
    if (!mine) return;
    if (state.can_buy) {
      const server = !!state.decision_deadline, dl = server ? new Date(state.decision_deadline).getTime() : Date.now() + 8000;
      $('btnEnd').hidden = false; $('buyCd').hidden = false;
      const upd = () => { $('buyCd').textContent = Math.max(0, Math.ceil((dl - Date.now()) / 1000)); };
      cdI = setInterval(upd, 250); upd();
      autoT = setTimeout(() => passTurn('Tempo esgotado: não comprou', ti), Math.max(0, dl - Date.now()) + (server ? 2500 : 0)); // server passes first when it can
    } else if (state.can_end_turn) {
      autoT = setTimeout(() => passTurn(undefined, ti), 2200);
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
    await rollDice(dice, meId);
    const card = await commit(r.game ? r : await getRoom(roomId), true, true);
    const me = state.players.find((p) => p.id === meId), t = state.board[me.position];
    toast(`Tirou ${dice[0] + dice[1]}. ${t.name}.` + (state.can_buy ? ` Comprar por ${brl(t.price)}?` : '') + (card ? ' ' + card : ''));
  });
  $('btnBuy').onclick = async () => { while (busy) await sleep(100); if (state.can_buy) buy(); };
  const buy = () => run(async () => { const r = await act('buy'); await commit(r.game ? r : await getRoom(roomId)); toast('Comprado!'); });
  $('btnEnd').onclick = () => passTurn();
  $('btnBuild').onclick = () => {
    const l = state.board.filter(canBuild);
    if (!l.length) return;
    sheet('Construir', `<ul class="blist">${l.map((t) => `<li style="--gc:var(--g-${t.group})"><i></i><span>${esc(t.name)}<small>${t.hotel ? '' : t.houses + ' casa(s)'}</small></span><button type="button" class="primary build-go" data-b="${t.position}">${buildLabel(t)}</button></li>`).join('')}</ul>`);
  };
  // build buttons inside the sheet (detail and list)
  $('sheetBody').addEventListener('click', async (e) => {
    const b = e.target.closest('.build-go');
    if (!b) return;
    const i = +b.dataset.b, list = !!b.closest('.blist');
    b.disabled = true;
    await buildAt(i);
    if (!list) showTile(i); else if (state.board.some(canBuild)) $('btnBuild').click(); else $('sheet').hidden = true;
  });
  $('btnMap').onclick = () => { const f = $('table').classList.toggle('flat'); $('btnMap').setAttribute('aria-pressed', f); if (f) camera(null); else followTurn(); setTimeout(placeAll, 1050); };

  let cardTimer = null;
  function sheet(title, html) { clearInterval(cardTimer); $('sheetTitle').textContent = title; $('sheetBody').innerHTML = html; $('sheet').hidden = false; }
  $('sheetClose').onclick = () => { clearInterval(cardTimer); $('sheet').hidden = true; };
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') $('sheet').hidden = true; });
  function showTile(i) {
    const t = state.board[i], o = state.players.find((p) => p.id === t.owner_id);
    if (!t.price) return sheet(CORNER[t.type] || (t.type === 'LUCK' ? luckName(t) : t.name), t.description ? `<p>${esc(t.description)}</p>` : '<p class="dim">Casa especial.</p>');
    const k = kindOf(t), list = PROPS[k] || [], idx = list.length ? hash(t.name) % list.length : -1, gc = t.type === 'PROPERTY' ? `var(--g-${t.group})` : '#38bdf8';
    const rent = t.type === 'PROPERTY' && t.rent.length > 5
      ? ['Aluguel base', '1 casa', '2 casas', '3 casas', '4 casas', 'Hotel'].map((l, n) => `<dt>${l}</dt><dd>${brl(t.rent[n])}</dd>`).join('')
      : (t.rent_base ? `<dt>Aluguel</dt><dd>${brl(t.rent_base)}</dd>` : '');
    const layers = idx < 0 ? '' : `<img class="on" src="${list[idx]}" alt="">${list.length > 1 ? '<img alt="">' : ''}`;
    sheet(t.name, `<div class="pc"><div class="pc-img" style="--gc:${gc}">${layers}${t.price ? `<span class="pc-pr">${brl(t.price)}</span>` : ''}<span class="pc-ic">${use(tileIcon(t, i))}</span><span class="pc-cap">${KIND_LABEL[k] || ''}${t.queen ? ' · ★ ícone da cidade' : ''}</span></div>${t.description ? `<p>${esc(t.description)}</p>` : ''}<dl><dt>Preço</dt><dd>${brl(t.price)}</dd>${rent}${t.build_cost ? `<dt>Casa custa</dt><dd>${brl(t.build_cost)}</dd>` : ''}${t.mortgage ? `<dt>Hipoteca</dt><dd>${brl(t.mortgage)}</dd>` : ''}<dt>Dono</dt><dd>${o ? esc(o.name) : 'Livre'}</dd></dl>${t.type === 'PROPERTY' && t.owner_id === meId ? `<button type="button" class="primary build-go" data-b="${i}" ${canBuild(t) ? '' : 'disabled'}>${buildLabel(t)}</button>${canBuild(t) ? '' : `<p class="dim">${buildWhy(t)}</p>`}` : ''}</div>`);
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
    $('tradeMsg').textContent = 'O outro jogador ou o bot decide: aceita ou recusa na hora.'; $('tradeMsg').classList.remove('bad');
    tradeOpener = document.activeElement; fillTrade(); vt(() => { $('trade').hidden = false; $('tradeTo').focus(); });
  }
  function closeTrade() { vt(() => { $('trade').hidden = true; if (tradeOpener && tradeOpener.focus) tradeOpener.focus(); }); }
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
      try {
        const cash0 = state.players.find((p) => p.id === meId).balance;
        const r = await api('/api/banco3d/room/trade', body);
        await commit(r.game ? r : await getRoom(roomId));
        // a bot answers on the spot and the reply carries no verdict: infer it from who owns the tiles / how much cash moved
        const tl = state.board, moved = body.give.props.length || body.want.props.length
          ? body.want.props.every((i) => tl[i].owner_id === meId) && body.give.props.every((i) => tl[i].owner_id === body.to_player_id)
          : state.players.find((p) => p.id === meId).balance !== cash0;
        if (r.pending_trade || r.trade_status === 'pending') { closeTrade(); toast('Proposta enviada. Aguardando resposta.'); }
        else if (r.trade_status ? r.trade_status === 'accepted' : moved) { closeTrade(); toast('Troca feita!'); } // older servers send no verdict: fall back to ownership
        else { $('tradeMsg').textContent = r.reason || r.trade_reason || 'O jogador recusou a proposta. Ajuste a oferta e tente de novo.'; $('tradeMsg').classList.add('bad'); }
      }
      catch (x) { $('tradeMsg').textContent = x.message; $('tradeMsg').classList.add('bad'); }
    });
  };

  // ---------- match config modal ----------
  const CFG_KEY = 'banco3d:config';
  const DEFAULT_CFG = { city: '', players: 4, balance: 1500, bots: 0, difficulty: 'normal', turn: 30, match: 0, bail: true, triple: true, freecard: true };
  let cfg = { ...DEFAULT_CFG };
  try { cfg = { ...cfg, ...JSON.parse(localStorage.getItem(CFG_KEY)) }; } catch { /* first run */ }
  let cfgOpener = null;
  const cf = () => $('cfgForm');
  function fillCfg() {
    const f = cf();
    f.city.value = $('fHome').city.value;
    f.balance.value = cfg.balance;
    for (const k of ['players', 'bots', 'difficulty', 'turn', 'match']) f.querySelectorAll(`[name=${k}]`).forEach((r) => (r.checked = String(r.value) === String(cfg[k])));
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
  function openCfg() { cfgOpener = document.activeElement; fillCfg(); vt(() => { $('cfg').hidden = false; cf().city.focus(); }); }
  function closeCfg() { vt(() => { $('cfg').hidden = true; if (cfgOpener && cfgOpener.focus) cfgOpener.focus(); }); }
  cf().addEventListener('change', syncBots);
  $('bCfg').onclick = openCfg;
  $('cfgX').onclick = $('cfgCancel').onclick = closeCfg;
  $('cfg').addEventListener('keydown', (e) => { if (e.key === 'Escape') closeCfg(); });
  cf().onsubmit = (e) => {
    e.preventDefault();
    const f = cf();
    cfg = { players: +f.players.value, balance: Math.min(10000, Math.max(500, +f.balance.value || 1500)), bots: +f.bots.value, difficulty: f.difficulty.value, turn: +f.turn.value, match: +f.match.value, bail: f.bail.checked, triple: f.triple.checked, freecard: f.freecard.checked };
    try { localStorage.setItem(CFG_KEY, JSON.stringify(cfg)); } catch { /* private mode */ }
    $('fHome').city.value = f.city.value.trim();
    closeCfg();
  };
  const cfgPayload = () => ({ max_players: cfg.players, bots: { count: cfg.bots, difficulty: cfg.difficulty }, starting_balance: cfg.balance, turn_seconds: cfg.turn, match_minutes: cfg.match, rules: { jail_bail: cfg.bail, jail_triple_doubles: cfg.triple, get_out_free_card: cfg.freecard } });

  // ---------- match clock (top HUD) ----------
  const mmss = (s) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
  function clockTick() {
    const end = state && state.status === 'in_progress' && state.match_end ? Date.parse(state.match_end) : NaN, el = $('matchClock');
    el.hidden = !(end > 0);
    if (el.hidden) return;
    const left = Math.max(0, Math.ceil((end - Date.now()) / 1000));
    const txt = mmss(left);
    if ($('clockTxt').textContent !== txt) $('clockTxt').textContent = txt;
    el.classList.toggle('low', left <= 60);
  }
  setInterval(clockTick, 500);

  // ---------- modals: options menu, rules, leave confirmation, final ranking ----------
  function modal(id, focusId) {
    const el = $(id); let opener = null;
    const m = { open() { opener = document.activeElement; vt(() => { el.hidden = false; $(focusId).focus(); }); }, close() { vt(() => { el.hidden = true; if (opener && opener.focus) opener.focus(); }); } };
    el.addEventListener('keydown', (e) => { if (e.key === 'Escape' && id !== 'final') m.close(); });
    el.addEventListener('mousedown', (e) => { if (e.target === el && id !== 'final') m.close(); });
    return m;
  }
  const mMenu = modal('menu', 'btnMotion'), mRules = modal('rules', 'rulesOk'), mLeave = modal('leave', 'leaveNo'), mFinal = modal('final', 'finalHome');
  $('btnMenu').onclick = mMenu.open;
  $('menuX').onclick = mMenu.close;
  $('btnRules').onclick = () => { mMenu.close(); mRules.open(); };
  $('rulesX').onclick = $('rulesOk').onclick = mRules.close;
  $('btnLeave').onclick = () => { mMenu.close(); mLeave.open(); };
  $('leaveNo').onclick = mLeave.close;

  // back to the hub: forget the room, stop everything that polls or animates
  function resetLocal() {
    const id = roomId;
    clearInterval(timer); stopAuto();
    if (id) forget(id);
    roomId = meId = raw = state = null; busy = polling = false; lastTurn = null; boardBuilt = false; localVoting = null; myVote = -1; starting = false;
    seenEv.clear(); seenEv.primed = false; seenCard = undefined; cardEvSeen = false; cardShown.clear(); cardQ.length = 0; cardClose && cardClose();
    prevPos.clear(); pawnPos.clear(); mineSig = ''; finalShown = false;
    ['menu', 'rules', 'leave', 'final', 'sheet', 'trade'].forEach((x) => { $(x).hidden = true; });
    $('cardFx').hidden = true; $('matchClock').hidden = true;
    const u = new URL(location.href); u.searchParams.delete('sala'); history.replaceState(null, '', u);
    setView('home');
  }
  $('leaveYes').onclick = async () => {
    $('leaveYes').disabled = true;
    try { await api('/api/banco3d/room/leave', { room_id: roomId, player_id: meId }); } catch { /* room gone or already out: leave locally anyway */ }
    $('leaveYes').disabled = false;
    resetLocal();
  };
  $('finalHome').onclick = resetLocal;

  let finalShown = false;
  async function showFinal(over) {
    if (finalShown) return;
    finalShown = true;
    while (cardBusy || cardQ.length) await sleep(200); // let the last card play first
    if (!roomId) return;
    const list = over && (over.ranking || over.standings || over.results);
    const wid = (over && (over.winner_id || over.winner)) || (state.players.find((p) => p.name === state.winner) || {}).id, wp = state.players.find((p) => p.id === wid);
    let rank = Array.isArray(list) && list.length
      ? list.map((x) => { const p = state.players.find((q) => q.id === (x.player_id || x.id)) || {}; return { name: x.name || p.name || 'Jogador', color: p.color, bankrupt: p.bankrupt, id: p.id, worth: x.net_worth ?? x.networth ?? x.worth ?? x.patrimonio ?? x.total ?? p.worth ?? 0 }; })
      : state.players.map((p) => ({ name: p.name, color: p.color, bankrupt: p.bankrupt, id: p.id, worth: p.worth })).sort((a, b) => (b.id === wid) - (a.id === wid) || a.bankrupt - b.bankrupt || b.worth - a.worth); // the winner leads even on a tie
    const winner = (wp && wp.name) || (over && over.winner_name) || state.winner || (rank[0] && rank[0].name) || '';
    $('finalWin').textContent = winner ? `${wp && wp.id === meId ? 'Você venceu' : winner + ' venceu'} a partida!` : 'A partida terminou.';
    $('finalRank').innerHTML = rank.map((r, i) => `<li class="${i === 0 ? 'first' : ''}" style="--c:${esc(r.color || '#888')}"><span class="pos">${i + 1}º</span><i></i><b>${esc(r.name)}${r.id === meId ? ' (você)' : ''}${r.bankrupt ? ' · falido' : ''}</b><em>${brl(r.worth)}</em></li>`).join('');
    mFinal.open();
  }

  // ---------- lobby / voting / room flow ----------
  let curView = 'home'; // logical view: the DOM follows one frame later when a view transition runs
  const setView = (v) => {
    curView = v;
    vt(() => {
      $('start').dataset.view = v; $('start').hidden = v === 'game';
      if (v === 'game') { const t = $('table'); t.classList.remove('enter'); void t.offsetWidth; t.classList.add('enter'); } // flat -> tilt entrance
    });
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
        if (curView !== 'game') { setView('game'); boardBuilt = false; }
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
  addEventListener('resize', () => { if (state) { placeAll(); layoutDice(); } });

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
        const sp = { 0: ['GO', 'Início'], 10: ['TRAVEL', 'Viagem'], 20: ['JAIL', 'Ilha Perdida'], 30: ['SPECIAL_TAX', 'Imposto Especial'], 4: ['TAX', 'Imposto'], 38: ['TAX', 'Imposto'], 7: ['CHANCE', 'Sorte'], 22: ['CHANCE', 'Sorte'], 17: ['CHEST', 'Revés'], 33: ['CHEST', 'Revés'] };
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
      room.events = room.events || [];
      if (me.position < d1 + d2) { me.money += 200; room.events.push({ type: 'salary', player_id: me.id, player_name: me.name, amount: 200, message: `${me.name} recebeu R$ 200 ao passar pela Saída`, timestamp: 's' + Date.now() }); }
      if (tt === 'CHANCE' || tt === 'CHEST') { const dl = [200, -100, 50][Math.floor(Math.random() * 3)]; me.money += dl; g.turn_index = (g.turn_index || 0) + 1; g.last_card_result = { card: { id: 'c' + g.turn_index, deck: tt, title: tt === 'CHANCE' ? 'Feira de artesanato lotada' : 'Conserto do bonde', text: 'Você ganhou destaque no bairro e a cidade reconhece o esforço.' }, player_id: me.id, money_delta: dl }; room.events.push({ type: 'card', player_id: me.id, deck: tt, title: g.last_card_result.card.title, text: g.last_card_result.card.text, amount: dl, timestamp: 'c' + Date.now() }); }
      return ok({ ...room, dice: { die1: d1, die2: d2 } });
    }
    if (P.endsWith('/room/__own')) { room.board.tiles.forEach((t) => { if (t.group === b.group && t.type === 'PROPERTY') { t.owner_id = me.id; if (!me.properties.includes(t.index)) me.properties.push(t.index); } }); return ok(room); }
    if (P.endsWith('/room/build')) { const t = room.board.tiles[b.tile_index]; t.houses = (t.houses || 0) + 1; me.money -= t.house_cost || 50; return ok(room); }
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
        if (room.board.tiles[p.position].type === 'CHANCE') room.events.push({ type: 'card', player_id: p.id, deck: 'CHANCE', title: 'Bônus de fim de ano', text: 'O banco paga um bônus a quem circula pela cidade.', amount: 50, timestamp: 'cb' + Date.now() + p.id });
        if (Math.random() < .5) { const a = 20 * (1 + Math.floor(Math.random() * 4)); p.money -= a; g.players[0].money += a; room.events.push({ type: 'rent', player_id: p.id, player_name: p.name, value: a, message: `${p.name} pagou R$ ${a} de aluguel`, timestamp: Date.now() + 'r' + p.id }); }
      });
      room.events = room.events.slice(-30); g.dice_rolled_this_turn = false; g.turn_index = (g.turn_index || 0) + 1;
      return ok(room);
    }
    return no('Indisponível no modo demo');
  }
})();
