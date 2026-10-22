(() => {
  const $ = (id) => document.getElementById(id);
  const qs = new URLSearchParams(location.search);
  const API = qs.get('api') || '';
  const DEMO = qs.has('demo');
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const brl = (n) => '$ ' + Number(n).toLocaleString('pt-BR');
  const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

  let gameId = null, state = null, meId = 1, busy = false;
  const pawns = new Map();
  const prevPos = new Map();

  // ---------- API (with offline demo engine for visual checks: ?demo) ----------
  async function api(path, body) {
    if (DEMO) return demoApi(path, body);
    const res = await fetch(API + path, {
      method: body ? 'POST' : 'GET',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify({ game_id: gameId, ...body }) : undefined,
      credentials: 'include',
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || data.message || 'Erro ' + res.status);
    return data;
  }

  // ---------- board ----------
  const STATION_ICONS = ['bolt', 'drop', 'wifi', 'train'];
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

  const CORNER = { START: 'Saída', JAIL: 'Prisão', PARKING: 'Visitante', GO_TO_JAIL: 'Vá para a Cadeia' };
  function buildBoard() {
    const board = $('board');
    const tiles = state.board, N = tiles.length, q = N / 4, S = q + 1;
    const tracks = `1.7fr repeat(${q - 1}, 1fr) 1.7fr`;
    board.style.gridTemplateColumns = tracks;
    board.style.gridTemplateRows = tracks;
    board.style.setProperty('--span', S - 2);
    board.innerHTML = '';
    let sn = 0;
    tiles.forEach((t, i) => {
      const [r, c] = cell(i, q);
      const corner = i % q === 0;
      const el = document.createElement('button');
      el.type = 'button';
      el.className = 'tile ' + (corner ? 'corner ' : '') + t.type.toLowerCase().replace('_', '') + (t.type === 'PROPERTY' ? ' prop' : '');
      el.dataset.i = i;
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
    pawns.clear();
    prevPos.clear();
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
    state.players.forEach((p, i) => { if (!p.bankrupt) placePawn(p, p.position, i); prevPos.set(p.id, p.position); });
  }
  async function walk(p, from, to) {
    const N = state.board.length, i = state.players.indexOf(p);
    for (let pos = from; pos !== to;) { pos = (pos + 1) % N; placePawn(p, pos, i); await sleep(200); }
  }

  // ---------- render ----------
  function render() {
    const pl = $('players');
    pl.innerHTML = state.players.map((p) =>
      `<div class="pl${p.id === state.current_turn ? ' turn' : ''}${p.bankrupt ? ' out' : ''}" style="--c:${esc(p.color)}"><div class="av">${use('user')}</div><div><b>${esc(p.name)}</b><span>${brl(p.balance)}</span></div>${p.id === state.current_turn ? '<span class="crown" aria-label="Sua vez">●</span>' : ''}</div>`).join('');
    $('cityName').textContent = state.city || 'Sua cidade';
    const mine = state.current_turn === meId && state.status === 'in_progress';
    const cur = state.players.find((p) => p.id === state.current_turn);
    $('statusText').textContent = state.status === 'finished' ? `Fim! ${state.winner || ''} venceu` : mine ? 'Sua vez' : `Vez de ${cur ? cur.name : '…'}`;
    $('btnRoll').disabled = busy || !(mine && state.can_roll);
    $('btnBuy').disabled = busy || !(mine && state.can_buy);
    $('btnEnd').disabled = busy || !(mine && state.can_end_turn);
    $('btnBuild').disabled = busy || !mine;
    $('btnTrade').disabled = busy;
    state.board.forEach((t, i) => {
      const el = $('board').children[i];
      const o = state.players.find((p) => p.id === t.owner_id);
      el.classList.toggle('owned', !!o);
      if (o) el.style.setProperty('--oc', o.color);
      el.querySelector('.hs').innerHTML = t.hotel ? '<i class="h"></i>' : '<i></i>'.repeat(t.houses || 0);
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
      const d = $(id), [rx, ry] = FACE[dice[k]];
      d.classList.remove('roll'); void d.offsetWidth; d.classList.add('roll');
      d.style.transform = `rotateX(${rx + 720 + k * 360}deg) rotateY(${ry + 720}deg)`;
    });
    await sleep(1000);
  }

  // ---------- actions ----------
  async function run(fn) {
    if (busy) return;
    busy = true; render();
    try { await fn(); } catch (e) { toast(e.message); }
    busy = false; if (state) render();
  }
  async function applyState(next, mover) {
    const old = new Map(prevPos);
    state = next;
    for (const p of state.players) {
      const from = old.get(p.id);
      if (from !== undefined && from !== p.position && !p.bankrupt) await walk(p, from, p.position);
      else if (!pawns.has(p.id)) placePawn(p, p.position, state.players.indexOf(p));
      prevPos.set(p.id, p.position);
    }
  }

  $('btnRoll').onclick = () => run(async () => {
    const r = await api('/api/banco/roll', { player_id: meId });
    const prev = state.players.find((p) => p.id === meId).position;
    await rollDice(r.dice);
    const me = r.state.players.find((p) => p.id === meId);
    if (me.position !== prev || r.passed_start) { const keep = r.state; state = { ...state, board: keep.board }; await walk(state.players.find((p) => p.id === meId), prev, me.position); }
    state = r.state; prevPos.set(meId, me.position); render();
    toast(r.message || r.event_log);
  });
  $('btnBuy').onclick = () => run(async () => {
    const r = await api('/api/banco/buy', { player_id: meId });
    state = r.state || r; render(); toast(r.message || 'Comprado!');
  });
  $('btnEnd').onclick = () => run(async () => {
    const r = await api('/api/banco/end-turn', { player_id: meId });
    for (const line of r.bot_actions || []) toast(line);
    await applyState(r.state);
    render();
  });
  $('btnMap').onclick = () => { const f = $('table').classList.toggle('flat'); $('btnMap').setAttribute('aria-pressed', f); setTimeout(placeAll, 720); };

  // ---------- sheets ----------
  function sheet(title, html) { $('sheetTitle').textContent = title; $('sheetBody').innerHTML = html; $('sheet').hidden = false; }
  $('sheetClose').onclick = () => ($('sheet').hidden = true);
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') $('sheet').hidden = true; });

  function showTile(i) {
    const t = state.board[i], o = state.players.find((p) => p.id === t.owner_id);
    sheet(t.name, `<p>${t.price ? 'Preço ' + brl(t.price) : t.type}</p>${t.rent_base ? `<p>Aluguel ${brl(t.rent_base)}</p>` : ''}<p>${o ? 'Dono: ' + esc(o.name) : t.price ? 'Livre' : ''}</p>`);
  }

  $('btnBuild').onclick = () => {
    const mine = state.players.find((p) => p.id === meId);
    const list = state.board.filter((t) => t.owner_id === meId && t.type === 'PROPERTY' && !t.hotel);
    sheet('Construir', list.length ? list.map((t) => `<div class="row"><span>${esc(t.name)}<br><span class="dim">${t.houses} casa(s) · ${brl(t.build_cost)}</span></span><button class="mini" data-b="${t.position}">Construir</button></div>`).join('') : '<p>Você ainda não tem imóveis.</p>');
    $('sheetBody').querySelectorAll('[data-b]').forEach((b) => (b.onclick = () => run(async () => {
      try { const r = await api('/api/banco/build', { player_id: meId, tile_pos: +b.dataset.b }); state = r.state || r; render(); toast(r.message || 'Construído!'); }
      finally { $('sheet').hidden = true; }
    })));
    void mine;
  };

  $('btnTrade').onclick = () => {
    const others = state.players.filter((p) => p.id !== meId && !p.bankrupt);
    sheet('Trocar', `<label class="dim" for="tP">Com quem</label><select id="tP">${others.map((p) => `<option value="${p.id}">${esc(p.name)}</option>`).join('')}</select>
      <label class="dim" for="tT">Imóvel que você quer</label><select id="tT"></select>
      <label class="dim" for="tC">Sua oferta (R$)</label><input id="tC" type="number" min="0" step="10" value="100" inputmode="numeric">
      <button class="mini" id="tGo" type="button">Propor troca</button> <button class="mini" id="tLoan" type="button" style="background:#334155">Empréstimo 200</button>`);
    const fill = () => {
      const p = state.players.find((x) => x.id === +$('tP').value);
      $('tT').innerHTML = (p ? p.properties : []).map((pos) => `<option value="${pos}">${esc(state.board[pos].name)}</option>`).join('') || '<option value="">Sem imóveis</option>';
    };
    $('tP').onchange = fill; fill();
    $('tGo').onclick = () => run(async () => {
      const r = await api('/api/banco/negotiate', { action: 'trade', from_player_id: meId, to_player_id: +$('tP').value, requested_props: $('tT').value ? [+$('tT').value] : [], offered_props: [], cash_offer: +$('tC').value || 0 });
      if (r.state) state = r.state; render(); toast(r.message || (r.accepted ? 'Aceito!' : 'Recusado'));
    });
    $('tLoan').onclick = () => run(async () => {
      const r = await api('/api/banco/negotiate', { action: 'loan', player_id: meId, loan_amount: 200 });
      if (r.state) state = r.state; render(); toast(r.message || 'Empréstimo');
    });
  };

  // ---------- start ----------
  $('startForm').onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target; $('startErr').textContent = '';
    try {
      const r = await api('/api/banco/start', { city: f.city.value.trim(), human_name: f.name.value.trim(), bots_count: +f.bots.value, difficulty: 'normal', starting_balance: 1500 });
      gameId = r.game_id; state = r.state;
      meId = (state.players.find((p) => !p.is_bot) || state.players[0]).id;
      $('start').hidden = true; buildBoard(); render(); requestAnimationFrame(placeAll);
      toast('O jogo está pronto!');
    } catch (err) { $('startErr').textContent = err.message; }
  };
  addEventListener('resize', () => state && placeAll());

  // ---------- demo engine ----------
  const DEMO_NAMES = ['Rua das Flores', 'Rua do Sol', 'Rua Aurora', 'Rua dos Ipês', 'Estação de Energia', 'Avenida Central', 'Avenida Paulista', 'Avenida Brasil', 'Avenida Atlântica', 'Rua 6 de Novembro', 'Rua Sete de Setembro', 'Rua Marechal Deodoro', 'Rua Floriano Peixoto', 'Estação de Água', 'Praça da República', 'Praça XV', 'Praça Mauá', 'Praça Tiradentes', 'Rua da Liberdade', 'Rua Galvão Bueno', 'Rua 25 de Março', 'Rua Direita', 'Estação de Trem', 'Rua Augusta', 'Rua Oscar Freire', 'Rua Haddock Lobo', 'Rua da Consolação', 'Rua Carlos Gomes', 'Rua Uruguai', 'Rua Rio Branco', 'Rua da Bahia', 'Estação de Internet', 'Rua Prudente de Morais', 'Rua Barata Ribeiro', 'Rua Visconde de Pirajá', 'Rua XV de Novembro'];
  let D;
  function demoApi(path, body) {
    const G = ['red', 'yellow', 'green', 'dark_blue', 'orange', 'pink'];
    if (path.endsWith('/start')) {
      const N = 40, names = DEMO_NAMES.slice();
      const board = Array.from({ length: N }, (_, i) => {
        const t = { position: i, type: 'PROPERTY', name: '', group: G[Math.floor(i / 7) % 6], price: 60 + i * 10, rent_base: 4 + i, build_cost: 50, owner_id: 0, houses: 0, hotel: false };
        if (i === 0) Object.assign(t, { type: 'START', name: 'Saída', price: 0 });
        else if (i === 10) Object.assign(t, { type: 'JAIL', name: 'Prisão', price: 0 });
        else if (i === 20) Object.assign(t, { type: 'PARKING', name: 'Visitante', price: 0 });
        else if (i === 30) Object.assign(t, { type: 'GO_TO_JAIL', name: 'Vá para a Cadeia', price: 0 });
        else if (i % 5 === 0) Object.assign(t, { type: 'STATION', name: names.shift(), price: 200, group: 'station' });
        else if (i === 4 || i === 38) Object.assign(t, { type: 'TAX', name: 'Imposto', price: 0 });
        else if (i % 7 === 2) Object.assign(t, { type: 'LUCK', name: 'Sorte', price: 0 });
        else t.name = names.shift() || 'Rua ' + i;
        return t;
      });
      const cols = ['#2f6bff', '#22c55e', '#facc15', '#ef4444'];
      const players = Array.from({ length: 1 + body.bots_count }, (_, k) => ({ id: k + 1, name: k ? ['Gabriel', 'Larissa', 'Pedro'][k - 1] : body.human_name, is_bot: !!k, color: cols[k], balance: 1500, position: 0, bankrupt: false, properties: [] }));
      D = { id: 'demo', city: body.city, players, board, current_turn: 1, status: 'in_progress', can_roll: true, can_buy: false, can_end_turn: false, last_dice: [1, 1], history: [] };
      return Promise.resolve({ game_id: 'demo', state: structuredClone(D) });
    }
    const me = D.players[0], snap = () => structuredClone(D);
    if (path.endsWith('/roll')) {
      const d = [1 + Math.floor(Math.random() * 6), 1 + Math.floor(Math.random() * 6)], from = me.position;
      me.position = (me.position + d[0] + d[1]) % 40;
      const t = D.board[me.position];
      D.can_roll = false; D.can_end_turn = true; D.can_buy = t.price > 0 && !t.owner_id && t.type !== 'TAX';
      const msg = `Você caiu em ${t.name}.` + (D.can_buy ? ` Comprar por ${brl(t.price)}?` : '');
      return Promise.resolve({ dice: d, total: d[0] + d[1], passed_start: me.position < from, message: msg, state: snap() });
    }
    if (path.endsWith('/buy')) {
      const t = D.board[me.position]; t.owner_id = 1; me.balance -= t.price; me.properties.push(t.position); D.can_buy = false;
      return Promise.resolve({ message: `Comprou ${t.name}`, state: snap() });
    }
    if (path.endsWith('/end-turn')) {
      D.players.slice(1).forEach((b) => { b.position = (b.position + 2 + Math.floor(Math.random() * 10)) % 40; });
      D.can_roll = true; D.can_end_turn = false; D.can_buy = false;
      return Promise.resolve({ bot_actions: ['Os adversários jogaram.'], state: snap() });
    }
    return Promise.reject(new Error('Indisponível no modo demo'));
  }
  if (DEMO) { $('startForm').requestSubmit(); }
})();
