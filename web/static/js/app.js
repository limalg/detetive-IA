/**
 * Detetive IA - Core Client Application
 * Noir Interactive Game & PWA Controller
 */

(() => {
  // Application State
  const state = {
    currentUser: null,
    currentCase: null,
    suspects: [],
    clues: [],
    activeSuspect: null,
    chatHistories: {}, // suspectId -> array of { sender, message, timestamp }
    suspectStress: {}, // suspectId -> integer 0-100
    currentRoom: null,
    gameMode: 'solo', // 'solo' | 'multi'
    persona: { name: '', role: '', secret: '' },
    deferredPrompt: null
  };

  // DOM Elements
  const el = {
    authSection: document.getElementById('auth-section'),
    lobbySection: document.getElementById('lobby-section'),
    gameSection: document.getElementById('game-section'),
    authForm: document.getElementById('auth-form'),
    authAlert: document.getElementById('auth-alert'),
    tabLogin: document.getElementById('tab-login'),
    tabRegister: document.getElementById('tab-register'),
    authSubmitBtn: document.getElementById('auth-submit-btn'),
    usernameInput: document.getElementById('username'),
    passwordInput: document.getElementById('password'),
    userDisplay: document.getElementById('user-display'),
    logoutBtn: document.getElementById('logout-btn'),
    networkStatus: document.getElementById('network-status'),
    pwaInstallBtn: document.getElementById('pwa-install-btn'),

    // Lobby Elements
    btnModeSolo: document.getElementById('btn-mode-solo'),
    btnModeMulti: document.getElementById('btn-mode-multi'),
    personaName: document.getElementById('persona-name'),
    personaRole: document.getElementById('persona-role'),
    personaSecret: document.getElementById('persona-secret'),
    btnRandomPersona: document.getElementById('btn-random-persona'),
    multiplayerRoomArea: document.getElementById('multiplayer-room-area'),
    btnCreateRoom: document.getElementById('btn-create-room'),
    roomCodeInput: document.getElementById('room-code-input'),
    btnJoinRoom: document.getElementById('btn-join-room'),
    roomWaitingArea: document.getElementById('room-waiting-area'),
    roomDisplayCode: document.getElementById('room-display-code'),
    btnCopyRoomLink: document.getElementById('btn-copy-room-link'),
    playersList: document.getElementById('players-list'),
    btnStartGame: document.getElementById('btn-start-game'),

    // Navigation
    navTabBtns: document.querySelectorAll('.nav-tab-btn'),
    tabPanes: document.querySelectorAll('.tab-pane'),

    // Case Dossier
    caseTitle: document.getElementById('case-title'),
    caseVictim: document.getElementById('case-victim'),
    caseDesc: document.getElementById('case-desc'),
    caseStamp: document.getElementById('case-stamp'),
    investigationStatus: document.getElementById('investigation-status'),
    suspectsList: document.getElementById('suspects-list'),
    cluesList: document.getElementById('clues-list'),

    // Interrogation
    interrogationModal: document.getElementById('interrogation-modal'),
    closeInterrogationBtn: document.getElementById('close-interrogation-btn'),
    interrogateModalTitle: document.getElementById('interrogate-modal-title'),
    interrogateSuspectProfile: document.getElementById('interrogate-suspect-profile'),
    chatTranscript: document.getElementById('chat-transcript'),
    interrogationForm: document.getElementById('interrogation-form'),
    interrogationInput: document.getElementById('interrogation-input'),
    promptChips: document.querySelectorAll('.chip'),
    confrontClueSelect: document.getElementById('confront-clue-select'),
    suspectStressMeter: document.getElementById('suspect-stress-meter'),
    suspectStressLabel: document.getElementById('suspect-stress-label'),

    // Accusation
    accusationForm: document.getElementById('accusation-form'),
    accuseSuspect: document.getElementById('accuse-suspect'),
    accuseWeapon: document.getElementById('accuse-weapon'),
    accuseMotive: document.getElementById('accuse-motive'),
    verdictResult: document.getElementById('verdict-result')
  };

  let authMode = 'login'; // 'login' | 'register'

  // =========================================================================
  // PWA & Service Worker
  // =========================================================================
  function initPWA() {
    if ('serviceWorker' in navigator) {
      window.addEventListener('load', () => {
        navigator.serviceWorker.register('/static/sw.js')
          .then((reg) => console.log('[PWA] Service Worker registrado:', reg.scope))
          .catch((err) => console.warn('[PWA] Falha ao registrar Service Worker:', err));
      });
    }

    // Capture install prompt
    window.addEventListener('beforeinstallprompt', (e) => {
      e.preventDefault();
      state.deferredPrompt = e;
      if (el.pwaInstallBtn) {
        el.pwaInstallBtn.style.display = 'inline-flex';
      }
    });

    if (el.pwaInstallBtn) {
      el.pwaInstallBtn.addEventListener('click', async () => {
        if (!state.deferredPrompt) return;
        state.deferredPrompt.prompt();
        const { outcome } = await state.deferredPrompt.userChoice;
        console.log('[PWA] Escolha do usuário:', outcome);
        state.deferredPrompt = null;
        el.pwaInstallBtn.style.display = 'none';
      });
    }

    // Network status monitoring
    function updateOnlineStatus() {
      if (navigator.onLine) {
        el.networkStatus.classList.remove('offline');
        el.networkStatus.title = 'Conectado à Central';
      } else {
        el.networkStatus.classList.add('offline');
        el.networkStatus.title = 'Modo Offline - Comunicação restrita';
      }
    }
    window.addEventListener('online', updateOnlineStatus);
    window.addEventListener('offline', updateOnlineStatus);
    updateOnlineStatus();
  }

  // =========================================================================
  // API Client Helper
  // =========================================================================
  async function api(endpoint, options = {}) {
    const defaultHeaders = {
      'Content-Type': 'application/json'
    };

    try {
      const response = await fetch(endpoint, {
        ...options,
        headers: {
          ...defaultHeaders,
          ...(options.headers || {})
        }
      });

      const data = await response.json().catch(() => ({}));

      if (!response.ok) {
        throw new Error(data.error || `Erro HTTP ${response.status}`);
      }

      return data;
    } catch (err) {
      console.error(`[API Error] ${endpoint}:`, err);
      throw err;
    }
  }

  // =========================================================================
  // Authentication Flow
  // =========================================================================
  function setupAuthEvents() {
    el.tabLogin.addEventListener('click', () => setAuthMode('login'));
    el.tabRegister.addEventListener('click', () => setAuthMode('register'));

    el.authForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = el.usernameInput.value.trim();
      const password = el.passwordInput.value;

      if (!username || !password) {
        showAuthAlert('Preencha todos os campos da credencial.', 'error');
        return;
      }

      el.authSubmitBtn.disabled = true;
      el.authSubmitBtn.textContent = 'Processando...';

      try {
        if (authMode === 'register') {
          await api('/api/auth/register', {
            method: 'POST',
            body: JSON.stringify({ username, password })
          });
          showAuthAlert('Investigador cadastrado com sucesso! Realizando acesso...', 'success');
        }

        // Always authenticate
        const res = await api('/api/auth/login', {
          method: 'POST',
          body: JSON.stringify({ username, password })
        });

        state.currentUser = res.user || { username };
        showAuthAlert('Credenciais verificadas. Acessando quartel...', 'success');
        setTimeout(() => {
          showLobbyView();
        }, 500);
      } catch (err) {
        showAuthAlert(err.message || 'Falha na autenticação de segurança.', 'error');
      } finally {
        el.authSubmitBtn.disabled = false;
        el.authSubmitBtn.textContent = authMode === 'login' ? 'Acessar Registros' : 'Cadastrar Investigador';
      }
    });

    if (el.logoutBtn) {
      el.logoutBtn.addEventListener('click', async () => {
        try {
          await api('/api/auth/logout', { method: 'POST' });
        } catch (_) {}
        state.currentUser = null;
        showAuthView();
      });
    }
  }

  function setAuthMode(mode) {
    authMode = mode;
    hideAuthAlert();
    if (mode === 'login') {
      el.tabLogin.classList.add('active');
      el.tabRegister.classList.remove('active');
      el.authSubmitBtn.textContent = 'Acessar Registros';
    } else {
      el.tabRegister.classList.add('active');
      el.tabLogin.classList.remove('active');
      el.authSubmitBtn.textContent = 'Cadastrar Investigador';
    }
  }

  function showAuthAlert(msg, type) {
    el.authAlert.textContent = msg;
    el.authAlert.className = `auth-alert ${type}`;
  }

  function hideAuthAlert() {
    el.authAlert.className = 'auth-alert';
    el.authAlert.textContent = '';
  }

  function showAuthView() {
    el.authSection.style.display = 'block';
    if (el.lobbySection) el.lobbySection.style.display = 'none';
    el.gameSection.style.display = 'none';
    el.userDisplay.textContent = 'Não identificado';
    if (el.logoutBtn) el.logoutBtn.style.display = 'none';
    el.usernameInput.value = '';
    el.passwordInput.value = '';
    hideAuthAlert();
  }

  function showLobbyView() {
    el.authSection.style.display = 'none';
    if (el.lobbySection) el.lobbySection.style.display = 'block';
    el.gameSection.style.display = 'none';
    if (state.currentUser) {
      el.userDisplay.textContent = `Det. ${state.currentUser.username}`;
    }
    if (el.logoutBtn) el.logoutBtn.style.display = 'inline-flex';
    if (!state.persona.name) {
      generateRandomPersona();
    } else {
      setupDefaultSoloLobby();
    }
  }

  function showGameView() {
    el.authSection.style.display = 'none';
    if (el.lobbySection) el.lobbySection.style.display = 'none';
    el.gameSection.style.display = 'block';
    const personaTitle = state.persona && state.persona.name ? state.persona.name : (state.currentUser ? state.currentUser.username : 'Investigador');
    el.userDisplay.textContent = `Det. ${personaTitle}`;
    if (el.logoutBtn) el.logoutBtn.style.display = 'inline-flex';
  }

  async function checkActiveSession() {
    try {
      const res = await api('/api/auth/me');
      if (res && res.user) {
        state.currentUser = res.user;
        showLobbyView();
      } else {
        showAuthView();
      }
    } catch (_) {
      showAuthView();
    }
  }

  // =========================================================================
  // Persona & Lobby Noir System
  // =========================================================================
  const NOIR_NAMES = [
    "Sam Spade", "Vivian Rutledge", "Philip Marlowe", "Evelyn Mulwray",
    "Walter Neff", "Phyllis Dietrichson", "Harry Lime", "Brigid O'Shaughnessy",
    "Lew Archer", "Nora Charles", "Nick Charles", "Arthur Hastings"
  ];

  const NOIR_ROLES = [
    "Detetive Particular Cínico",
    "Jornalista Investigativo Forense",
    "Médica Legista e Toxicologista",
    "Magnata das Ferrovias & Industrial",
    "Advogado Criminalista do Submundo",
    "Pianista Noturna de Jazz",
    "Fotógrafo Policial de Cena de Crime",
    "Ex-Falsificador de Registros Cartoriais"
  ];

  const NOIR_SECRETS = [
    "Dívidas volumosas com agiotas ligados ao porto",
    "Já ocultei uma evidência no passado para poupar um inocente",
    "Conhecia a vítima no passado e guardava rancor pessoal",
    "Falsifiquei uma credencial oficial para entrar no caso",
    "Possuo a chave mestra dos armários da mansão",
    "Fui sócio secreto da vítima em exportações ilegais",
    "Estava nas sombras do jardim pouco antes do disparo fatal"
  ];

  function generateRandomPersona() {
    const name = NOIR_NAMES[Math.floor(Math.random() * NOIR_NAMES.length)];
    const role = NOIR_ROLES[Math.floor(Math.random() * NOIR_ROLES.length)];
    const secret = NOIR_SECRETS[Math.floor(Math.random() * NOIR_SECRETS.length)];

    state.persona = { name, role, secret };

    if (el.personaName) el.personaName.value = name;
    if (el.personaRole) el.personaRole.value = role;
    if (el.personaSecret) el.personaSecret.value = secret;

    if (el.btnRandomPersona) {
      el.btnRandomPersona.textContent = '🎲 Rolando...';
      setTimeout(() => {
        el.btnRandomPersona.textContent = '🎲 Sortear Persona';
      }, 300);
    }

    if (state.gameMode === 'solo') {
      setupDefaultSoloLobby();
    }
  }

  function savePersonaFromInputs() {
    state.persona = {
      name: el.personaName ? el.personaName.value.trim() : '',
      role: el.personaRole ? el.personaRole.value.trim() : '',
      secret: el.personaSecret ? el.personaSecret.value.trim() : ''
    };
  }

  function setupDefaultSoloLobby() {
    const pName = (el.personaName && el.personaName.value) || state.persona.name || 'Você';
    const pRole = (el.personaRole && el.personaRole.value) || state.persona.role || 'Investigador Principal';

    enterRoomLobby('SOLO', true, [
      { name: pName, role: pRole, is_host: true, is_bot: false },
      { name: 'Inspetor Vance (Bot)', role: 'Chefe do Departamento', is_host: false, is_bot: true },
      { name: 'Dra. Thorne (Bot)', role: 'Perita Balística', is_host: false, is_bot: true },
      { name: 'Miller (Bot)', role: 'Patrulheiro Noturno', is_host: false, is_bot: true }
    ]);
    if (el.btnStartGame) {
      el.btnStartGame.textContent = '🚀 Iniciar Caso Solo com Bots';
    }
  }

  function enterRoomLobby(code, isHost, players) {
    state.currentRoom = { code, isHost, players: players || [] };
    if (el.roomDisplayCode) el.roomDisplayCode.textContent = code;
    if (el.roomWaitingArea) el.roomWaitingArea.style.display = 'block';
    renderLobbyPlayers();
  }

  function renderLobbyPlayers() {
    if (!el.playersList || !state.currentRoom) return;
    el.playersList.innerHTML = '';
    const list = state.currentRoom.players || [];
    list.forEach((p) => {
      const chip = document.createElement('div');
      chip.className = `player-chip ${p.is_host ? 'is-host' : ''} ${p.is_bot ? 'is-bot' : ''}`;
      chip.innerHTML = `
        <span style="font-size: 1.1rem;">${p.is_bot ? '🤖' : '🕵️'}</span>
        <div>
          <div style="font-weight: bold; color: var(--text-main);">${escapeHtml(p.name)}</div>
          <div style="font-size: 0.75rem; color: var(--text-muted);">${escapeHtml(p.role || '')}</div>
        </div>
        ${p.is_host ? '<span class="player-badge-tag host">Host</span>' : (p.is_bot ? '<span class="player-badge-tag bot">Bot</span>' : '')}
      `;
      el.playersList.appendChild(chip);
    });
  }

  function setupLobbyEvents() {
    if (el.btnRandomPersona) {
      el.btnRandomPersona.addEventListener('click', generateRandomPersona);
    }

    if (el.personaName) {
      el.personaName.addEventListener('input', () => {
        savePersonaFromInputs();
        if (state.gameMode === 'solo') setupDefaultSoloLobby();
      });
    }

    // Mode Solo
    if (el.btnModeSolo) {
      el.btnModeSolo.addEventListener('click', () => {
        state.gameMode = 'solo';
        el.btnModeSolo.classList.add('active');
        if (el.btnModeMulti) el.btnModeMulti.classList.remove('active');
        if (el.multiplayerRoomArea) el.multiplayerRoomArea.style.display = 'none';
        setupDefaultSoloLobby();
      });
    }

    // Mode Multi
    if (el.btnModeMulti) {
      el.btnModeMulti.addEventListener('click', () => {
        state.gameMode = 'multi';
        el.btnModeMulti.classList.add('active');
        if (el.btnModeSolo) el.btnModeSolo.classList.remove('active');
        if (el.multiplayerRoomArea) el.multiplayerRoomArea.style.display = 'block';
        if (el.roomWaitingArea) el.roomWaitingArea.style.display = 'none';
        state.currentRoom = null;
      });
    }

    // Room code input sanitization
    if (el.roomCodeInput) {
      el.roomCodeInput.addEventListener('input', (e) => {
        e.target.value = e.target.value.toUpperCase().replace(/[^A-Z]/g, '').slice(0, 4);
      });
    }

    // Create Room
    if (el.btnCreateRoom) {
      el.btnCreateRoom.addEventListener('click', async () => {
        savePersonaFromInputs();
        const pName = state.persona.name || (state.currentUser ? state.currentUser.username : 'Investigador');
        const pRole = state.persona.role || 'Detetive Particular';
        const pSecret = state.persona.secret || '';

        try {
          const res = await api('/api/room/create', {
            method: 'POST',
            body: JSON.stringify({
              persona_name: pName,
              role: pRole,
              secret: pSecret
            })
          });
          enterRoomLobby(res.code || 'NOIR', true, res.players);
        } catch (_) {
          // Graceful fallback while room backend is active
          enterRoomLobby('NOIR', true, [
            { name: pName, role: pRole, is_host: true, is_bot: false },
            { name: 'Dr. Thorne (Bot)', role: 'Perito Balístico', is_host: false, is_bot: true },
            { name: 'Miller (Bot)', role: 'Patrulheiro Noturno', is_host: false, is_bot: true }
          ]);
        }
        if (el.btnStartGame) {
          el.btnStartGame.textContent = '🚨 Iniciar Investigação da Sala';
        }
      });
    }

    // Join Room
    if (el.btnJoinRoom) {
      el.btnJoinRoom.addEventListener('click', async () => {
        const code = (el.roomCodeInput.value || '').trim().toUpperCase();
        if (code.length !== 4) {
          alert('Por favor digite um código de 4 letras válido (Ex: NOIR)');
          return;
        }
        savePersonaFromInputs();
        const pName = state.persona.name || (state.currentUser ? state.currentUser.username : 'Investigador');
        const pRole = state.persona.role || 'Detetive Particular';
        const pSecret = state.persona.secret || '';

        try {
          const res = await api('/api/room/join', {
            method: 'POST',
            body: JSON.stringify({
              code: code,
              persona_name: pName,
              role: pRole,
              secret: pSecret
            })
          });
          enterRoomLobby(code, false, res.players);
        } catch (_) {
          // Graceful fallback
          enterRoomLobby(code, false, [
            { name: 'Host da Sala', role: 'Detetive Líder', is_host: true, is_bot: false },
            { name: pName, role: pRole, is_host: false, is_bot: false },
            { name: 'Inspetor Vance (Bot)', role: 'Chefe de Polícia', is_host: false, is_bot: true }
          ]);
        }
        if (el.btnStartGame) {
          el.btnStartGame.textContent = '🚨 Iniciar Investigação da Sala';
        }
      });
    }

    // Copy Room Link
    if (el.btnCopyRoomLink) {
      el.btnCopyRoomLink.addEventListener('click', () => {
        if (!state.currentRoom) return;
        const url = `${window.location.origin}/?room=${state.currentRoom.code}`;
        navigator.clipboard.writeText(url).then(() => {
          el.btnCopyRoomLink.textContent = '✅ Copiado!';
          setTimeout(() => {
            el.btnCopyRoomLink.textContent = '📋 Copiar Link';
          }, 1500);
        }).catch(() => {
          prompt('Copie o link da sala:', url);
        });
      });
    }

    // Start Game
    if (el.btnStartGame) {
      el.btnStartGame.addEventListener('click', async () => {
        savePersonaFromInputs();
        if (state.currentRoom && state.currentRoom.code && state.currentRoom.code !== 'SOLO') {
          try {
            await api(`/api/room/${state.currentRoom.code}/start`, { method: 'POST' });
          } catch (_) {}
        }
        showGameView();
        loadCaseData();
      });
    }
  }



  // =========================================================================
  // Game & Dossier Logic
  // =========================================================================
  async function loadCaseData() {
    try {
      const res = await api('/api/game/case');
      state.currentCase = res.case || res;
      state.suspects = res.suspects || [];
      state.clues = res.clues || [];

      renderDossier();
      renderSuspects();
      renderClues();
      populateAccusationOptions();
    } catch (err) {
      console.warn('[Game] Erro ao carregar caso da API, usando fallback:', err);
      // Fallback initial mystery seed if API endpoint in progress
      setupOfflineFallbackCase();
    }
  }

  function setupOfflineFallbackCase() {
    state.currentCase = {
      id: 1,
      title: 'O Enigma do Vale das Sombras',
      victim: 'Arthur Pendelton (Magnata das Ferrovias)',
      description: 'Na calada da noite chuvosa de 14 de Outubro, o corpo do Sr. Arthur Pendelton foi encontrado em seu escritório particular na Mansão Vale das Sombras. Três pessoas próximas estavam na residência. Uma arma foi encontrada abandonada.',
      status: 'in_progress'
    };

    state.suspects = [
      {
        id: 1,
        name: 'Dra. Helena Voss',
        role: 'Médica da Família',
        personality: 'Fria, analítica e calculista',
        alibi: 'Afirma que estava no laboratório organizando prescrições médicas na hora do crime.'
      },
      {
        id: 2,
        name: 'Victor Vance',
        role: 'Sobrinho e Herdeiro',
        personality: 'Ansioso, extravagante e afundado em dívidas de apostas',
        alibi: 'Disse que estava na adega degustando vinhos raros.'
      },
      {
        id: 3,
        name: 'Clarice Dubois',
        role: 'Governanta & Confidente',
        personality: 'Discreta, leal na aparência e observadora atenta',
        alibi: 'Alega que recolhia a louça no piso térreo quando ouviu um estrondo.'
      }
    ];

    state.clues = [
      {
        id: 1,
        location: 'Escritório Principal',
        name: 'Cálice com Resíduo de Cianeto',
        description: 'Frasco de cristal encontrado sobre a mesa, exalando odor amargo de amêndoas.'
      },
      {
        id: 2,
        location: 'Biblioteca',
        name: 'Testamento Rasgado',
        description: 'Pedaços de documento revelando que Arthur planejava deserdar seu único herdeiro.'
      },
      {
        id: 3,
        location: 'Jardim de Inverno',
        name: 'Pegada Úmida de Salto Fino',
        description: 'Marca no piso de mármore compatível com calçados femininos de gala.'
      }
    ];

    renderDossier();
    renderSuspects();
    renderClues();
    populateAccusationOptions();
  }

  function renderDossier() {
    if (!state.currentCase) return;
    el.caseTitle.textContent = state.currentCase.title;
    el.caseVictim.textContent = state.currentCase.victim;
    el.caseDesc.textContent = state.currentCase.description;

    const status = state.currentCase.status || 'in_progress';
    if (status === 'solved') {
      el.caseStamp.textContent = 'RESOLVIDO';
      el.caseStamp.className = 'case-stamp solved';
      el.investigationStatus.textContent = 'Caso Encerrado com Sucesso';
    } else if (status === 'failed') {
      el.caseStamp.textContent = 'FALHOU';
      el.caseStamp.className = 'case-stamp failed';
      el.investigationStatus.textContent = 'Assassino Escapou';
    } else {
      el.caseStamp.textContent = 'EM ANDAMENTO';
      el.caseStamp.className = 'case-stamp in-progress';
      el.investigationStatus.textContent = 'Investigação Ativa';
    }
  }

  function renderSuspects() {
    el.suspectsList.innerHTML = '';
    state.suspects.forEach((suspect) => {
      const card = document.createElement('div');
      card.className = 'card-noir';
      card.innerHTML = `
        <div>
          <div class="suspect-header">
            <div class="suspect-avatar">${suspect.name.charAt(0)}</div>
            <div>
              <div class="suspect-title">${escapeHtml(suspect.name)}</div>
              <div class="suspect-role">${escapeHtml(suspect.role)}</div>
            </div>
          </div>
          <div class="suspect-body">
            <p><strong>Perfil:</strong> ${escapeHtml(suspect.personality || 'Não documentado')}</p>
            <div class="suspect-alibi">
              <strong>Álibi:</strong> "${escapeHtml(suspect.alibi || 'Nenhum álibi declarado.')}"
            </div>
          </div>
        </div>
        <button class="btn btn-primary btn-interrogate" data-id="${suspect.id}" style="width: 100%; margin-top: 1rem;">
          Interrogar Suspeito
        </button>
      `;

      card.querySelector('.btn-interrogate').addEventListener('click', () => {
        openInterrogation(suspect);
      });

      el.suspectsList.appendChild(card);
    });
  }

  function renderClues() {
    el.cluesList.innerHTML = '';
    state.clues.forEach((clue) => {
      const card = document.createElement('div');
      card.className = 'card-noir';
      card.innerHTML = `
        <div>
          <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 0.75rem;">
            <div class="suspect-title" style="font-size: 1.05rem;">${escapeHtml(clue.name)}</div>
            <span class="clue-badge">${escapeHtml(clue.location)}</span>
          </div>
          <div class="suspect-body">
            <p>${escapeHtml(clue.description)}</p>
          </div>
        </div>
      `;
      el.cluesList.appendChild(card);
    });
  }

  function populateAccusationOptions() {
    // Populate suspects dropdown
    el.accuseSuspect.innerHTML = '<option value="">Selecione um suspeito...</option>';
    state.suspects.forEach((s) => {
      const opt = document.createElement('option');
      opt.value = s.id;
      opt.textContent = `${s.name} (${s.role})`;
      el.accuseSuspect.appendChild(opt);
    });

    // Populate weapons dropdown
    el.accuseWeapon.innerHTML = '<option value="">Selecione a arma...</option>';
    const weapons = [
      'Cálice com Resíduo de Cianeto',
      'Adaga Cerimonial de Prata',
      'Revólver Calibre .38 com Silenciador',
      'Castiçal Pesado de Bronze',
      'Veneno Estricnina no Whisky'
    ];
    weapons.forEach((w) => {
      const opt = document.createElement('option');
      opt.value = w;
      opt.textContent = w;
      el.accuseWeapon.appendChild(opt);
    });
  }

  function updateStressMeter(suspectId, stress) {
    if (!el.suspectStressMeter || !el.suspectStressLabel) return;
    const clamped = Math.max(0, Math.min(100, Math.round(stress || 0)));
    state.suspectStress[suspectId] = clamped;

    el.suspectStressMeter.style.width = `${clamped}%`;
    el.suspectStressMeter.className = 'stress-bar-fill';

    let label = `${clamped}% (Calmo / Calculista)`;
    if (clamped > 75) {
      el.suspectStressMeter.classList.add('panic');
      label = `${clamped}% (EM PÂNICO / ÁLIBI QUEBRADO!)`;
    } else if (clamped > 35) {
      el.suspectStressMeter.classList.add('nervous');
      label = `${clamped}% (Nervoso / Inseguro)`;
    } else {
      el.suspectStressMeter.classList.add('calm');
    }
    el.suspectStressLabel.textContent = label;
  }

  function updateConfrontClueOptions() {
    if (!el.confrontClueSelect) return;
    el.confrontClueSelect.innerHTML = '<option value="">Nenhuma evidência (Pergunta regular)</option>';
    (state.clues || []).forEach((clue) => {
      const opt = document.createElement('option');
      opt.value = clue.id;
      opt.textContent = `🔍 Pista: ${clue.name}`;
      el.confrontClueSelect.appendChild(opt);
    });
  }

  // =========================================================================
  // Interrogation Room
  // =========================================================================
  function openInterrogation(suspect) {
    state.activeSuspect = suspect;
    el.interrogateModalTitle.textContent = `Interrogatório: ${suspect.name}`;
    el.interrogateSuspectProfile.innerHTML = `
      <strong>Cargo:</strong> ${escapeHtml(suspect.role)} &bull; 
      <strong>Temperamento:</strong> ${escapeHtml(suspect.personality || 'Reservado')} &bull;
      <em>${escapeHtml(suspect.alibi)}</em>
    `;

    updateConfrontClueOptions();
    updateStressMeter(suspect.id, state.suspectStress[suspect.id] || 0);

    if (!state.chatHistories[suspect.id]) {
      state.chatHistories[suspect.id] = [
        {
          sender: 'suspect',
          message: `O que você quer de mim, detetive? Eu já disse aos seus guardas tudo o que sei. Não tenho nada a esconder.`
        }
      ];
    }

    renderChatTranscript();
    el.interrogationModal.classList.add('active');
    el.interrogationInput.focus();
  }

  function closeInterrogation() {
    el.interrogationModal.classList.remove('active');
    state.activeSuspect = null;
  }

  function renderChatTranscript() {
    if (!state.activeSuspect) return;
    const history = state.chatHistories[state.activeSuspect.id] || [];
    el.chatTranscript.innerHTML = '';

    history.forEach((item) => {
      const bubble = document.createElement('div');
      bubble.className = `chat-bubble ${item.sender}`;
      const senderLabel = item.sender === 'detective' ? `Det. ${state.persona.name || (state.currentUser ? state.currentUser.username : 'Detetive')}` : state.activeSuspect.name;
      bubble.innerHTML = `
        <div class="bubble-sender">${escapeHtml(senderLabel)}</div>
        <div class="bubble-text">${escapeHtml(item.message)}</div>
      `;
      el.chatTranscript.appendChild(bubble);
    });

    el.chatTranscript.scrollTop = el.chatTranscript.scrollHeight;
  }

  function setupInterrogationEvents() {
    el.closeInterrogationBtn.addEventListener('click', closeInterrogation);

    // Click outside modal card to close
    el.interrogationModal.addEventListener('click', (e) => {
      if (e.target === el.interrogationModal) {
        closeInterrogation();
      }
    });

    // Prompt chips
    el.promptChips.forEach((chip) => {
      chip.addEventListener('click', () => {
        const prompt = chip.getAttribute('data-prompt');
        el.interrogationInput.value = prompt;
        el.interrogationInput.focus();
      });
    });

    // Send question form with clue confrontation
    el.interrogationForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const question = el.interrogationInput.value.trim();
      if (!question || !state.activeSuspect) return;

      const suspectId = state.activeSuspect.id;
      const confrontClueVal = el.confrontClueSelect ? el.confrontClueSelect.value : '';
      const confrontClueId = confrontClueVal ? parseInt(confrontClueVal, 10) : null;

      // Add detective message (highlight confrontation if applied)
      let displayedQuestion = question;
      if (confrontClueId && el.confrontClueSelect && el.confrontClueSelect.selectedOptions[0]) {
        displayedQuestion = `[⚔️ CONFRONTANDO COM: ${el.confrontClueSelect.selectedOptions[0].text}] ${question}`;
      }

      state.chatHistories[suspectId].push({
        sender: 'detective',
        message: displayedQuestion
      });
      renderChatTranscript();
      el.interrogationInput.value = '';

      // Typing indicator
      const typingBubble = document.createElement('div');
      typingBubble.className = 'chat-bubble suspect';
      typingBubble.id = 'typing-indicator';
      typingBubble.innerHTML = `<div class="bubble-sender">${escapeHtml(state.activeSuspect.name)}</div><em>Digitando depoimento...</em>`;
      el.chatTranscript.appendChild(typingBubble);
      el.chatTranscript.scrollTop = el.chatTranscript.scrollHeight;

      const reqBody = {
        suspect_id: suspectId,
        question: question
      };
      if (confrontClueId) {
        reqBody.confront_clue_id = confrontClueId;
      }

      try {
        const res = await api('/api/game/interrogate', {
          method: 'POST',
          body: JSON.stringify(reqBody)
        });

        const reply = res.reply || res.message || 'O suspeito hesita, olha fixamente nos seus olhos e permanece em silêncio calculista.';
        removeTypingIndicator();
        state.chatHistories[suspectId].push({
          sender: 'suspect',
          message: reply
        });
        renderChatTranscript();

        const currentStress = state.suspectStress[suspectId] || 0;
        const newStress = typeof res.stress_level === 'number'
          ? res.stress_level
          : (currentStress + (confrontClueId ? 40 : 5));
        updateStressMeter(suspectId, newStress);
      } catch (err) {
        removeTypingIndicator();
        // Fallback scripted response for offline/fallback mode
        const fallbackReply = generateFallbackResponse(state.activeSuspect, question, confrontClueId);
        state.chatHistories[suspectId].push({
          sender: 'suspect',
          message: fallbackReply
        });
        renderChatTranscript();
        const currentStress = state.suspectStress[suspectId] || 0;
        const newStress = currentStress + (confrontClueId ? 40 : 10);
        updateStressMeter(suspectId, newStress);
      }

      if (el.confrontClueSelect) el.confrontClueSelect.value = '';
    });
  }


  function removeTypingIndicator() {
    const typing = document.getElementById('typing-indicator');
    if (typing) typing.remove();
  }

  function generateFallbackResponse(suspect, question) {
    const q = question.toLowerCase();
    if (q.includes('onde') || q.includes('álibi') || q.includes('alibi')) {
      return `Como já afirmei: ${suspect.alibi} Havia testemunhas que podem comprovar se o senhor fizer o seu trabalho.`;
    }
    if (q.includes('vítima') || q.includes('arthur') || q.includes('relação')) {
      return `Nossa relação era puramente profissional e formal. O Sr. Pendelton tinha muitos inimigos no ramo das ferrovias, detetive.`;
    }
    if (q.includes('arma') || q.includes('cianeto') || q.includes('veneno')) {
      return `Eu jamais tocaria em algo tão abominável. Se alguém envenenou Arthur, foi alguém com acesso irrestrito às salas privadas!`;
    }
    if (q.includes('segredo') || q.includes('mentira') || q.includes('confessar')) {
      return `Todos nesta casa guardam segredos, senhor Marlowe. Mas nenhum deles me torna um assassino a sangue frio.`;
    }
    return `Não tenho nada mais a declarar sobre isso. Sugiro que concentre suas suspeitas em quem realmente tinha motivos financeiros!`;
  }

  // =========================================================================
  // Accusation & Verdict
  // =========================================================================
  function setupAccusationEvents() {
    el.accusationForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const suspectId = parseInt(el.accuseSuspect.value, 10);
      const weapon = el.accuseWeapon.value;
      const motive = el.accuseMotive.value;

      if (!suspectId || !weapon || !motive) {
        alert('Selecione todos os elementos da acusação.');
        return;
      }

      const submitBtn = document.getElementById('submit-accusation-btn');
      submitBtn.disabled = true;
      submitBtn.textContent = 'Enviando Mandado ao Tribunal...';

      try {
        const res = await api('/api/game/accuse', {
          method: 'POST',
          body: JSON.stringify({
            case_id: state.currentCase ? state.currentCase.id : 1,
            suspect_id: suspectId,
            weapon: weapon,
            motive: motive
          })
        });

        displayVerdict(res);
      } catch (err) {
        // Fallback verdict resolution for test / mock
        const isCorrect = suspectId === 1 && weapon.includes('Cianeto');
        displayVerdict({
          is_correct: isCorrect,
          message: isCorrect
            ? 'Mandado aceito pelo Magistrado! A Dra. Helena Voss confessou sob pressão após a perícia encontrar traços de cianeto em seu anel de sinete!'
            : 'Acusação rejeitada por falta de provas contundentes! O suspeito foi liberado pelo juiz e o verdadeiro assassino fugiu da cidade.'
        });
      } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = '⚖️ Emitir Mandado de Prisão';
      }
    });
  }

  function displayVerdict(res) {
    el.verdictResult.style.display = 'block';
    const isCorrect = res.is_correct || res.status === 'solved' || res.solved;
    const bannerClass = isCorrect ? 'verdict-banner solved' : 'verdict-banner failed';
    const stampClass = isCorrect ? 'case-stamp solved' : 'case-stamp failed';
    const stampText = isCorrect ? 'RESOLVIDO' : 'FALHOU';

    el.verdictResult.innerHTML = `
      <div class="${bannerClass}">
        <h4 style="font-family: var(--font-typewriter); font-size: 1.4rem; margin-bottom: 0.5rem;">
          ${isCorrect ? '🏆 CASO RESOLVIDO COM LOUVOR!' : '❌ ACUSAÇÃO INCORRETA!'}
        </h4>
        <p style="font-size: 0.95rem; line-height: 1.6;">${escapeHtml(res.message || res.verdict || '')}</p>
      </div>
    `;

    el.caseStamp.textContent = stampText;
    el.caseStamp.className = stampClass;
    if (state.currentCase) {
      state.currentCase.status = isCorrect ? 'solved' : 'failed';
    }

    el.verdictResult.scrollIntoView({ behavior: 'smooth' });
  }

  // =========================================================================
  // Navigation Tabs
  // =========================================================================
  function setupNavEvents() {
    el.navTabBtns.forEach((btn) => {
      btn.addEventListener('click', () => {
        const tab = btn.getAttribute('data-tab');

        el.navTabBtns.forEach((b) => b.classList.remove('active'));
        btn.classList.add('active');

        el.tabPanes.forEach((pane) => {
          if (pane.id === `tab-content-${tab}`) {
            pane.style.display = 'block';
          } else {
            pane.style.display = 'none';
          }
        });
      });
    });
  }

  // =========================================================================
  // Utility
  // =========================================================================
  function escapeHtml(str) {
    if (!str) return '';
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // =========================================================================
  // App Initialization
  // =========================================================================
  function init() {
    initPWA();
    setupAuthEvents();
    setupLobbyEvents();
    setupNavEvents();
    setupInterrogationEvents();
    setupAccusationEvents();
    checkActiveSession();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
