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
    deferredPrompt: null
  };

  // DOM Elements
  const el = {
    authSection: document.getElementById('auth-section'),
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
        showAuthAlert('Credenciais verificadas. Carregando dossiê...', 'success');
        setTimeout(() => {
          showGameView();
          loadCaseData();
        }, 600);
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
    el.gameSection.style.display = 'none';
    el.userDisplay.textContent = 'Não identificado';
    if (el.logoutBtn) el.logoutBtn.style.display = 'none';
    el.usernameInput.value = '';
    el.passwordInput.value = '';
    hideAuthAlert();
  }

  function showGameView() {
    el.authSection.style.display = 'none';
    el.gameSection.style.display = 'block';
    el.userDisplay.textContent = `Det. ${state.currentUser.username}`;
    if (el.logoutBtn) el.logoutBtn.style.display = 'inline-flex';
  }

  async function checkActiveSession() {
    try {
      const res = await api('/api/auth/me');
      if (res && res.user) {
        state.currentUser = res.user;
        showGameView();
        loadCaseData();
      } else {
        showAuthView();
      }
    } catch (_) {
      showAuthView();
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
      const senderLabel = item.sender === 'detective' ? `Det. ${state.currentUser ? state.currentUser.username : 'Detetive'}` : state.activeSuspect.name;
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

    // Send question form
    el.interrogationForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const question = el.interrogationInput.value.trim();
      if (!question || !state.activeSuspect) return;

      const suspectId = state.activeSuspect.id;

      // Add detective message
      state.chatHistories[suspectId].push({
        sender: 'detective',
        message: question
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

      try {
        const res = await api('/api/game/interrogate', {
          method: 'POST',
          body: JSON.stringify({
            suspect_id: suspectId,
            question: question
          })
        });

        const reply = res.reply || res.message || 'O suspeito hesita, olha fixamente nos seus olhos e permanece em silêncio calculista.';
        removeTypingIndicator();
        state.chatHistories[suspectId].push({
          sender: 'suspect',
          message: reply
        });
        renderChatTranscript();
      } catch (err) {
        removeTypingIndicator();
        // Fallback scripted response for offline/fallback mode
        const fallbackReply = generateFallbackResponse(state.activeSuspect, question);
        state.chatHistories[suspectId].push({
          sender: 'suspect',
          message: fallbackReply
        });
        renderChatTranscript();
      }
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
